package client

import (
	"context"
	"strconv"
)

type ResourceGroup struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// A string to string property map.
	Props        map[string]string `json:"props,omitempty"`
	SelectFilter AutoSelectFilter  `json:"select_filter,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
}

type ResourceGroupModify struct {
	Description string `json:"description,omitempty"`
	// A string to string property map.
	OverrideProps    map[string]string `json:"override_props,omitempty"`
	DeleteProps      []string          `json:"delete_props,omitempty"`
	DeleteNamespaces []string          `json:"delete_namespaces,omitempty"`
	SelectFilter     AutoSelectFilter  `json:"select_filter,omitempty"`
}

type ResourceGroupSpawn struct {
	// name of the resulting resource-definition
	ResourceDefinitionName string `json:"resource_definition_name,omitempty"`
	// sizes (in kib) of the resulting volume-definitions
	VolumeSizes []int64 `json:"volume_sizes,omitempty"`
	// If false, the length of the vlm_sizes has to match the number of volume-groups or an error is returned.  If true and there are more vlm_sizes than volume-groups, the additional volume-definitions will simply have no pre-set properties (i.e. \"empty\" volume-definitions) If true and there are less vlm_sizes than volume-groups, the additional volume-groups won't be used.  If the count of vlm_sizes matches the number of volume-groups, this \"partial\" parameter has no effect.
	Partial bool `json:"partial,omitempty"`
	// If true, the spawn command will only create the resource-definition with the volume-definitions but will not perform an auto-place, even if it is configured.
	DefinitionsOnly bool `json:"definitions_only,omitempty"`
}

type VolumeGroup struct {
	VolumeNumber int32 `json:"volume_number,omitempty"`
	// A string to string property map.
	Props map[string]string `json:"props,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
}

// custom code

// ResourceGroupService is the service that deals with resource group related tasks.
type ResourceGroupService struct {
	client *Client
}

// GetAll lists all resource-groups
func (n *ResourceGroupService) GetAll(ctx context.Context, opts ...*ListOpts) ([]ResourceGroup, error) {
	var resGrps []ResourceGroup
	_, err := n.client.doGET(ctx, "/v1/resource-groups", &resGrps, opts...)
	return resGrps, err
}

// Get return information about a resource-defintion
func (n *ResourceGroupService) Get(ctx context.Context, resGrpName string, opts ...*ListOpts) (ResourceGroup, error) {
	var resGrp ResourceGroup
	_, err := n.client.doGET(ctx, "/v1/resource-groups/"+resGrpName, &resGrp, opts...)
	return resGrp, err
}

// Create adds a new resource-group
func (n *ResourceGroupService) Create(ctx context.Context, resGrp ResourceGroup) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-groups", resGrp)
	return err
}

// Modify allows to modify a resource-group
func (n *ResourceGroupService) Modify(ctx context.Context, resGrpName string, props GenericPropsModify) error {
	_, err := n.client.doPUT(ctx, "/v1/resource-groups/"+resGrpName, props)
	return err
}

// Delete deletes a resource-group
func (n *ResourceGroupService) Delete(ctx context.Context, resGrpName string) error {
	_, err := n.client.doDELETE(ctx, "/v1/resource-groups/"+resGrpName, nil)
	return err
}

// Spawn creates a new resource-definition and auto-deploys if configured to do so
func (n *ResourceGroupService) Spawn(ctx context.Context, resGrpName string, resGrpSpwn ResourceGroupSpawn) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-groups/"+resGrpName+"/spawn", resGrpSpwn)
	return err
}

// GetVolumeGroups lists all volume-groups for a resource-group
func (n *ResourceGroupService) GetVolumeGroups(ctx context.Context, resGrpName string, opts ...*ListOpts) ([]VolumeGroup, error) {
	var volGrps []VolumeGroup
	_, err := n.client.doGET(ctx, "/v1/resource-groups/"+resGrpName+"/volume-groups", &volGrps, opts...)
	return volGrps, err
}

// GetVolumeGroup lists a volume-group for a resource-group
func (n *ResourceGroupService) GetVolumeGroup(ctx context.Context, resGrpName string, volNr int, opts ...*ListOpts) (VolumeGroup, error) {
	var volGrp VolumeGroup
	_, err := n.client.doGET(ctx, "/v1/resource-groups/"+resGrpName+"/volume-groups/"+strconv.Itoa(volNr), &volGrp, opts...)
	return volGrp, err
}

// Create adds a new volume-group to a resource-group
func (n *ResourceGroupService) CreateVolumeGroup(ctx context.Context, resGrpName string, volGrp VolumeGroup) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-groups/"+resGrpName+"/volume-groups", volGrp)
	return err
}

// Modify allows to modify a volume-group of a resource-group
func (n *ResourceGroupService) ModifyVolumeGroup(ctx context.Context, resGrpName string, volNr int, props GenericPropsModify) error {
	_, err := n.client.doPUT(ctx, "/v1/resource-groups/"+resGrpName+"/volume-groups/"+strconv.Itoa(volNr), props)
	return err
}

func (n *ResourceGroupService) DeleteVolumeGroup(ctx context.Context, resGrpName string, volNr int) error {
	_, err := n.client.doDELETE(ctx, "/v1/resource-groups/"+resGrpName+"/volume-groups/"+strconv.Itoa(volNr), nil)
	return err
}
