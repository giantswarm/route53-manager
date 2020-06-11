package key

import "fmt"

const (
	TagCluster = "giantswarm.io/cluster"
)

func BaseDomain(clusterID string, hostedZoneName string) string {
	return fmt.Sprintf("%s.%s", clusterID, hostedZoneName)
}

func EtcdENIDNSName(baseDomain string, index int) string {
	return fmt.Sprintf("etcd%d.%s", index+1, baseDomain)
}

func EtcdEniResourceName(index int) string {
	return fmt.Sprintf("EtcdEniDNSRecordSet%d", index+1)
}
