package quantify

import (
	"fmt"
	"reflect"

	"cloud.google.com/go/compute/metadata"
)

const (
	cloudResourceFieldTag = "cloud_resource_field"

	resourceNameGlobal       = "global"
	resourceNameGkeContainer = "gke_container"
	resourceNameGceInstance  = "gce_instance"
)

var (
	ErrInvalidResourceFieldType = fmt.Errorf("field tagged as %s isn't of type string", cloudResourceFieldTag)
)

type Resource interface {
	GetName() string
}

type Global struct {
	ProjectId string `cloud_resource_field:"project_id"`
}

type GceInstance struct {
	ProjectId  string `cloud_resource_field:"project_id"`
	InstanceId string `cloud_resource_field:"instance_id"`
	Zone       string `cloud_resource_field:"zone"`
}

type GkeContainer struct {
	ProjectId     string `cloud_resource_field:"project_id"`
	ClusterName   string `cloud_resource_field:"cluster_name"`
	InstanceId    string `cloud_resource_field:"instance_id"`
	Zone          string `cloud_resource_field:"zone"`
	NamespaceId   string `cloud_resource_field:"namespace_id"`
	PodId         string `cloud_resource_field:"pod_id"`
	ContainerName string `cloud_resource_field:"container_name"`
}

func (g *Global) GetName() string {
	return resourceNameGlobal
}

func (gi *GceInstance) GetName() string {
	return resourceNameGceInstance
}

func (gc *GkeContainer) GetName() string {
	return resourceNameGkeContainer
}

func flatten(v Resource) (map[string]string, error) {

	result := make(map[string]string)

	rv := reflect.ValueOf(v)
	t := rv.Type()

	for i := 0; i < t.NumField(); i++ {

		field, ok := t.Field(i).Tag.Lookup(cloudResourceFieldTag)
		if !ok {
			continue
		}

		if rv.Field(i).Kind() != reflect.String {
			return nil, ErrInvalidResourceFieldType
		}

		value := rv.Field(i).String()

		if value == "" {
			continue
		}

		result[field] = value
	}

	return result, nil
}

func DetectProjectId() string {
	projectId, _ := metadata.ProjectID()
	return projectId
}

func DetectZone() string {
	zone, _ := metadata.Zone()
	return zone
}

func DetectInstanceId() string {
	instanceId, _ := metadata.InstanceID()
	return instanceId
}

func DetectGkeClusterName() string {
	name, _ := metadata.InstanceAttributeValue("cluster-name")
	return name
}
