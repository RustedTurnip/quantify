package quantify

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

type mockResource struct{}

func (mr *mockResource) GetName() string {
	return "mock_resource"
}

func TestOptionWithResourceType(t *testing.T) {

	tests := []struct {
		name               string
		input              Resource
		expectedQuantifier *Quantifier
		expectedError      error
	}{
		{
			name: "normal input",
			input: &ResourceGenericNode{
				ProjectId: "test-project",
				Location:  "test-location",
				Namespace: "test-namespace",
				NodeId:    "test-node-id",
			},
			expectedQuantifier: &Quantifier{
				resourceName: "generic_node",
				resourceLabels: map[string]string{
					"project_id": "test-project",
					"location":   "test-location",
					"namespace":  "test-namespace",
					"node_id":    "test-node-id",
				},
			},
			expectedError: nil,
		},
		{
			name:               "missing project_id",
			input:              &mockResource{},
			expectedQuantifier: &Quantifier{},
			expectedError:      errors.New("missing required project_id resource label"),
		},
	}

	for _, test := range tests {

		fn := OptionWithResourceType(Resource(test.input))
		client := &Quantifier{}

		assert.Equalf(t, test.expectedError, fn(client), "%s failed", test.name)
		assert.Equalf(t, test.expectedQuantifier, client, "%s failed", test.name)
	}
}
