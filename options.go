package cloud_metrics

import (
	"errors"
	"os"

	"cloud.google.com/go/compute/metadata"
)

type ResourceType string
type ResourceField string

const (
	ResourceGlobal       ResourceType = "global"
	ResourceGkeContainer ResourceType = "gke_container"
	ResourceGceInstance  ResourceType = "gce_Instance"

	FieldProjectId     ResourceField = "project_id"
	FieldClusterName   ResourceField = "cluster_name"
	FieldInstanceId    ResourceField = "instance_id"
	FieldZone          ResourceField = "zone"
	FieldNamespaceId   ResourceField = "namespace_id"
	FieldPodId         ResourceField = "pod_id"
	FieldContainerName ResourceField = "container_name"
)

// Option defines a function for supplying the Reporter constructor with certain
// configurations.
type Option func(reporter *Reporter) error

// configure will use the default options specified under getters to attempt to
// populate any remaining ResourceFields that weren't supplied as options to
// the Reporter constructor.
func (r *Reporter) configure() error {

	for option, _ := range configurations[r.resourceType] {

		// if field has already been applied (through option in constructor), skip
		if _, ok := r.resourceConfig[string(option)]; ok {
			continue
		}

		if err := getters[option](r); err != nil {
			return err
		}
	}

	return nil
}

var (
	// configurations is a map linking a ResourceType to it's expected ResourceField(s). These
	// configurations are as specified in the link below:
	//
	// https://cloud.google.com/monitoring/custom-metrics#which-resource
	configurations = map[ResourceType]map[ResourceField]interface{}{

		// https://cloud.google.com/monitoring/api/resources#tag_global
		ResourceGlobal: {
			FieldProjectId: struct{}{},
		},

		// https://cloud.google.com/monitoring/api/resources#tag_gce_instance
		ResourceGceInstance: {
			FieldProjectId:  struct{}{},
			FieldInstanceId: struct{}{},
			FieldZone:       struct{}{},
		},

		// https://cloud.google.com/monitoring/api/resources#tag_gke_container
		ResourceGkeContainer: {
			FieldProjectId:     struct{}{},
			FieldClusterName:   struct{}{},
			FieldInstanceId:    struct{}{},
			FieldZone:          struct{}{},
			FieldNamespaceId:   struct{}{},
			FieldPodId:         struct{}{},
			FieldContainerName: struct{}{},
		},
	}

	// getters maps each ResourceField to it's default "getter" option.
	getters = map[ResourceField]Option{

		FieldProjectId: func(r *Reporter) error {

			pId, err := metadata.ProjectID()
			if err != nil {
				return err
			}

			r.resourceConfig[string(FieldProjectId)] = pId
			return nil
		},

		FieldClusterName: func(r *Reporter) error {

			cn, err := metadata.InstanceAttributeValue("cluster-name")
			if err != nil {
				return err
			}

			r.resourceConfig[string(FieldClusterName)] = cn
			return nil
		},

		FieldInstanceId: func(r *Reporter) error {
			iId, err := metadata.InstanceID()
			if err != nil {
				return err
			}

			r.resourceConfig[string(FieldInstanceId)] = iId
			return nil
		},

		FieldZone: func(r *Reporter) error {
			z, err := metadata.InstanceAttributeValue("cluster-location")
			if err != nil {
				return err
			}

			r.resourceConfig[string(FieldZone)] = z
			return nil
		},

		FieldNamespaceId: func(r *Reporter) error {
			ns := os.Getenv("NAMESPACE")
			if ns == "" {
				return errors.New("missing field") // TODO extract error
			}

			r.resourceConfig[string(FieldNamespaceId)] = ns
			return nil
		},

		FieldPodId: func(r *Reporter) error {
			pId := os.Getenv("POD_ID")
			if pId == "" {
				return errors.New("missing field") // TODO extract error
			}

			r.resourceConfig[string(FieldPodId)] = pId
			return nil
		},

		FieldContainerName: func(r *Reporter) error {
			cn := os.Getenv("CONTAINER_NAME")
			if cn == "" {
				return errors.New("missing field") // TODO extract error
			}

			r.resourceConfig[string(FieldContainerName)] = cn
			return nil
		},
	}
)

// addConfigOption is used to add a provided ResourceField to the Reporter's config.
//
// If the ResourceField isn't required for the Reporter's config, the option will
// be ignored.
func (r *Reporter) addConfigOption(field ResourceField, value string) {
	if _, ok := configurations[r.resourceType][field]; ok {
		r.resourceConfig[string(field)] = value
	}
}

func WithOptionProjectId(projectId string) Option {
	return func(r *Reporter) error {
		r.addConfigOption(FieldProjectId, projectId)
		return nil
	}
}

func WithOptionClusterName(clusterName string) Option {
	return func(r *Reporter) error {
		r.addConfigOption(FieldClusterName, clusterName)
		return nil
	}
}

func WithOptionInstanceId(instanceId string) Option {
	return func(r *Reporter) error {
		r.addConfigOption(FieldInstanceId, instanceId)
		return nil
	}
}

func WithOptionZone(zone string) Option {
	return func(r *Reporter) error {
		r.addConfigOption(FieldZone, zone)
		return nil
	}
}

func WithOptionNamespaceId(namespaceId string) Option {
	return func(r *Reporter) error {
		r.addConfigOption(FieldNamespaceId, namespaceId)
		return nil
	}
}

func WithOptionPodId(podId string) Option {
	return func(r *Reporter) error {
		r.addConfigOption(FieldPodId, podId)
		return nil
	}
}

func WithOptionContainerName(containerName string) Option {
	return func(r *Reporter) error {
		r.addConfigOption(FieldContainerName, containerName)
		return nil
	}
}
