package kubernetes

import (
	kc "cke/kubernetes/cluster"
)

//IngressItem ingress的item
type IngressItem struct {
	Name string         `json:"name"`
	Port kc.IngressPort `json:"port"`
}

//TODO fix bug
/*func buildIngressItem(ingress  *kc.Ingress) *IngressItem {
	return &IngressItem{
		Name:       ingress.Name,
		Port:       ingress.Port,
	}
}

func buildIngressItems(Ingresses  kc.Ingresses) []*IngressItem {
	items := make([]*IngressItem, 0, 10)

	for _, n := range Ingresses {
		items = append(items, buildIngressItem(n))
	}

	return items
}*/
