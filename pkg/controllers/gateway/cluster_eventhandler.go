package gateway

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/clusterSecret"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
)

type ClusterEventHandler struct {
	client client.Client
}

var _ handler.EventHandler = &ClusterEventHandler{}

// Create implements handler.EventHandler
func (eh *ClusterEventHandler) Create(ctx context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(ctx, e.Object, q)
}

// Delete implements handler.EventHandler
func (eh *ClusterEventHandler) Delete(ctx context.Context, e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(ctx, e.Object, q)
}

// Generic implements handler.EventHandler
func (eh *ClusterEventHandler) Generic(ctx context.Context, e event.GenericEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(ctx, e.Object, q)
}

// Update implements handler.EventHandler
func (eh *ClusterEventHandler) Update(ctx context.Context, e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(ctx, e.ObjectNew, q)
}

func (eh *ClusterEventHandler) enqueueForObject(ctx context.Context, obj v1.Object, q workqueue.RateLimitingInterface) {
	if !clusterSecret.IsClusterSecret(obj) {
		return
	}

	gateways, err := eh.getGatewaysFor(ctx, obj.(*corev1.Secret))
	if err != nil {
		log.Log.Error(err, "failed to get gateways when enqueueing from cluster secret")
		return
	}

	for _, gateway := range gateways {
		log.Log.Info(fmt.Sprintf("Enqueing reconciliation from secret update to gateway/%s", gateway.Name))
		q.Add(ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(&gateway),
		})
	}
}

func (eh *ClusterEventHandler) getGatewaysFor(ctx context.Context, secret *corev1.Secret) ([]gatewayapiv1.Gateway, error) {

	gateways := &gatewayapiv1.GatewayList{}
	if err := eh.client.List(ctx, gateways, &client.ListOptions{Namespace: secret.Namespace}); err != nil {
		return nil, err
	}

	return slice.Filter(gateways.Items, func(gateway gatewayapiv1.Gateway) bool {
		for _, l := range gateway.Spec.Listeners {
			if l.Protocol != gatewayapiv1.HTTPSProtocolType || l.TLS == nil {
				continue
			}

			for _, ts := range l.TLS.CertificateRefs {
				if ts.Name == gatewayapiv1.ObjectName(secret.Name) && *ts.Namespace == gatewayapiv1.Namespace(secret.Namespace) {
					return true
				}
			}

		}
		return false
	}), nil
}
