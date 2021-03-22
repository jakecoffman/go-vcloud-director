/*
 * Copyright 2021 VMware, Inc.  All rights reserved.  Licensed under the Apache v2 License.
 */

package govcd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/go-vcloud-director/v2/types/v56"
	"github.com/vmware/go-vcloud-director/v2/util"
)

type Vdc struct {
	Vdc    *types.Vdc
	client *Client
}

func NewVdc(cli *Client) *Vdc {
	return &Vdc{
		Vdc:    new(types.Vdc),
		client: cli,
	}
}

// Gets a vapp with a specific url vappHREF
func (vdc *Vdc) getVdcVAppbyHREF(ctx context.Context, vappHREF *url.URL) (*VApp, error) {
	vapp := NewVApp(vdc.client)

	_, err := vdc.client.ExecuteRequest(ctx, vappHREF.String(), http.MethodGet,
		"", "error retrieving VApp: %s", nil, vapp.VApp)

	return vapp, err
}

// Undeploys every vapp in the vdc
func (vdc *Vdc) undeployAllVdcVApps(ctx context.Context) error {
	err := vdc.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("error refreshing vdc: %s", err)
	}
	for _, resents := range vdc.Vdc.ResourceEntities {
		for _, resent := range resents.ResourceEntity {
			if resent.Type == "application/vnd.vmware.vcloud.vApp+xml" {
				vappHREF, err := url.Parse(resent.HREF)
				if err != nil {
					return err
				}
				vapp, err := vdc.getVdcVAppbyHREF(ctx, vappHREF)
				if err != nil {
					return fmt.Errorf("error retrieving vapp with url: %s and with error %s", vappHREF.Path, err)
				}
				task, err := vapp.Undeploy(ctx)
				if err != nil {
					return err
				}
				if task == (Task{}) {
					continue
				}
				err = task.WaitTaskCompletion(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Removes all vapps in the vdc
func (vdc *Vdc) removeAllVdcVApps(ctx context.Context) error {
	err := vdc.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("error refreshing vdc: %s", err)
	}
	for _, resents := range vdc.Vdc.ResourceEntities {
		for _, resent := range resents.ResourceEntity {
			if resent.Type == "application/vnd.vmware.vcloud.vApp+xml" {
				vappHREF, err := url.Parse(resent.HREF)
				if err != nil {
					return err
				}
				vapp, err := vdc.getVdcVAppbyHREF(ctx, vappHREF)
				if err != nil {
					return fmt.Errorf("error retrieving vapp with url: %s and with error %s", vappHREF.Path, err)
				}
				task, err := vapp.Delete(ctx)
				if err != nil {
					return fmt.Errorf("error deleting vapp: %s", err)
				}
				err = task.WaitTaskCompletion(ctx)
				if err != nil {
					return fmt.Errorf("couldn't finish removing vapp %s", err)
				}
			}
		}
	}
	return nil
}

func (vdc *Vdc) Refresh(ctx context.Context) error {

	if vdc.Vdc.HREF == "" {
		return fmt.Errorf("cannot refresh, Object is empty")
	}

	// Empty struct before a new unmarshal, otherwise we end up with duplicate
	// elements in slices.
	unmarshalledVdc := &types.Vdc{}

	_, err := vdc.client.ExecuteRequest(ctx, vdc.Vdc.HREF, http.MethodGet,
		"", "error refreshing vDC: %s", nil, unmarshalledVdc)
	if err != nil {
		return err
	}

	vdc.Vdc = unmarshalledVdc

	// The request was successful
	return nil
}

// Deletes the vdc, returning an error of the vCD call fails.
// API Documentation: https://code.vmware.com/apis/220/vcloud#/doc/doc/operations/DELETE-Vdc.html
func (vdc *Vdc) Delete(ctx context.Context, force bool, recursive bool) (Task, error) {
	util.Logger.Printf("[TRACE] Vdc.Delete - deleting VDC with force: %t, recursive: %t", force, recursive)

	if vdc.Vdc.HREF == "" {
		return Task{}, fmt.Errorf("cannot delete, Object is empty")
	}

	vdcUrl, err := url.ParseRequestURI(vdc.Vdc.HREF)
	if err != nil {
		return Task{}, fmt.Errorf("error parsing vdc url: %s", err)
	}

	req := vdc.client.NewRequest(ctx, map[string]string{
		"force":     strconv.FormatBool(force),
		"recursive": strconv.FormatBool(recursive),
	}, http.MethodDelete, *vdcUrl, nil)
	resp, err := checkResp(vdc.client.Http.Do(req))
	if err != nil {
		return Task{}, fmt.Errorf("error deleting vdc: %s", err)
	}
	task := NewTask(vdc.client)
	if err = decodeBody(types.BodyTypeXML, resp, task.Task); err != nil {
		return Task{}, fmt.Errorf("error decoding task response: %s", err)
	}
	if task.Task.Status == "error" {
		return Task{}, fmt.Errorf("vdc not properly destroyed")
	}
	return *task, nil
}

// Deletes the vdc and waits for the asynchronous task to complete.
func (vdc *Vdc) DeleteWait(ctx context.Context, force bool, recursive bool) error {
	task, err := vdc.Delete(ctx, force, recursive)
	if err != nil {
		return err
	}
	err = task.WaitTaskCompletion(ctx)
	if err != nil {
		return fmt.Errorf("couldn't finish removing vdc %s", err)
	}
	return nil
}

// Deprecated: use GetOrgVdcNetworkByName
func (vdc *Vdc) FindVDCNetwork(ctx context.Context, network string) (OrgVDCNetwork, error) {

	err := vdc.Refresh(ctx)
	if err != nil {
		return OrgVDCNetwork{}, fmt.Errorf("error refreshing vdc: %s", err)
	}
	for _, an := range vdc.Vdc.AvailableNetworks {
		for _, reference := range an.Network {
			if reference.Name == network {
				orgNet := NewOrgVDCNetwork(vdc.client)

				_, err := vdc.client.ExecuteRequest(ctx, reference.HREF, http.MethodGet,
					"", "error retrieving org vdc network: %s", nil, orgNet.OrgVDCNetwork)

				// The request was successful
				return *orgNet, err

			}
		}
	}

	return OrgVDCNetwork{}, fmt.Errorf("can't find VDC Network: %s", network)
}

// GetOrgVdcNetworkByHref returns an Org VDC Network reference if the network HREF matches an existing one.
// If no valid external network is found, it returns a nil Network reference and an error
func (vdc *Vdc) GetOrgVdcNetworkByHref(ctx context.Context, href string) (*OrgVDCNetwork, error) {

	orgNet := NewOrgVDCNetwork(vdc.client)

	_, err := vdc.client.ExecuteRequest(ctx, href, http.MethodGet,
		"", "error retrieving org vdc network: %s", nil, orgNet.OrgVDCNetwork)

	// The request was successful
	return orgNet, err
}

// GetOrgVdcNetworkByName returns an Org VDC Network reference if the network name matches an existing one.
// If no valid external network is found, it returns a nil Network reference and an error
func (vdc *Vdc) GetOrgVdcNetworkByName(ctx context.Context, name string, refresh bool) (*OrgVDCNetwork, error) {
	if refresh {
		err := vdc.Refresh(ctx)
		if err != nil {
			return nil, fmt.Errorf("error refreshing vdc: %s", err)
		}
	}
	for _, an := range vdc.Vdc.AvailableNetworks {
		for _, reference := range an.Network {
			if reference.Name == name {
				return vdc.GetOrgVdcNetworkByHref(ctx, reference.HREF)
			}
		}
	}

	return nil, ErrorEntityNotFound
}

// GetOrgVdcNetworkById returns an Org VDC Network reference if the network ID matches an existing one.
// If no valid external network is found, it returns a nil Network reference and an error
func (vdc *Vdc) GetOrgVdcNetworkById(ctx context.Context, id string, refresh bool) (*OrgVDCNetwork, error) {
	if refresh {
		err := vdc.Refresh(ctx)
		if err != nil {
			return nil, fmt.Errorf("error refreshing vdc: %s", err)
		}
	}
	for _, an := range vdc.Vdc.AvailableNetworks {
		for _, reference := range an.Network {
			// Some versions of vCD do not return an ID in the network reference
			// We use equalIds to overcome this issue
			if equalIds(id, reference.ID, reference.HREF) {
				return vdc.GetOrgVdcNetworkByHref(ctx, reference.HREF)
			}
		}
	}

	return nil, ErrorEntityNotFound
}

// GetOrgVdcNetworkByNameOrId returns a VDC Network reference if either the network name or ID matches an existing one.
// If no valid external network is found, it returns a nil ExternalNetwork reference and an error
func (vdc *Vdc) GetOrgVdcNetworkByNameOrId(ctx context.Context, identifier string, refresh bool) (*OrgVDCNetwork, error) {
	getByName := func(name string, refresh bool) (interface{}, error) {
		return vdc.GetOrgVdcNetworkByName(ctx, name, refresh)
	}
	getById := func(id string, refresh bool) (interface{}, error) { return vdc.GetOrgVdcNetworkById(ctx, id, refresh) }
	entity, err := getEntityByNameOrId(getByName, getById, identifier, false)
	if entity == nil {
		return nil, err
	}
	return entity.(*OrgVDCNetwork), err
}

func (vdc *Vdc) FindStorageProfileReference(ctx context.Context, name string) (types.Reference, error) {

	err := vdc.Refresh(ctx)
	if err != nil {
		return types.Reference{}, fmt.Errorf("error refreshing vdc: %s", err)
	}
	for _, sp := range vdc.Vdc.VdcStorageProfiles.VdcStorageProfile {
		if sp.Name == name {
			return types.Reference{HREF: sp.HREF, Name: sp.Name, ID: sp.ID}, nil
		}
	}
	return types.Reference{}, fmt.Errorf("can't find any VDC Storage_profiles")
}

func (vdc *Vdc) GetDefaultStorageProfileReference(ctx context.Context, storageprofiles *types.QueryResultRecordsType) (types.Reference, error) {

	err := vdc.Refresh(ctx)
	if err != nil {
		return types.Reference{}, fmt.Errorf("error refreshing vdc: %s", err)
	}
	for _, spr := range storageprofiles.OrgVdcStorageProfileRecord {
		if spr.IsDefaultStorageProfile {
			return types.Reference{HREF: spr.HREF, Name: spr.Name}, nil
		}
	}
	return types.Reference{}, fmt.Errorf("can't find Default VDC Storage_profile")
}

// Deprecated: use GetEdgeGatewayByName
func (vdc *Vdc) FindEdgeGateway(ctx context.Context, edgegateway string) (EdgeGateway, error) {

	err := vdc.Refresh(ctx)
	if err != nil {
		return EdgeGateway{}, fmt.Errorf("error refreshing vdc: %s", err)
	}
	for _, av := range vdc.Vdc.Link {
		if av.Rel == "edgeGateways" && av.Type == types.MimeQueryRecords {

			query := new(types.QueryResultEdgeGatewayRecordsType)

			_, err := vdc.client.ExecuteRequest(ctx, av.HREF, http.MethodGet,
				"", "error querying edge gateways: %s", nil, query)
			if err != nil {
				return EdgeGateway{}, err
			}

			var href string

			for _, edge := range query.EdgeGatewayRecord {
				if edge.Name == edgegateway {
					href = edge.HREF
				}
			}

			if href == "" {
				return EdgeGateway{}, fmt.Errorf("can't find edge gateway with name: %s", edgegateway)
			}

			edge := NewEdgeGateway(vdc.client)

			_, err = vdc.client.ExecuteRequest(ctx, href, http.MethodGet,
				"", "error retrieving edge gateway: %s", nil, edge.EdgeGateway)

			// TODO - remove this if a solution is found or once 9.7 is deprecated
			// vCD 9.7 has a bug and sometimes it fails to retrieve edge gateway with weird error.
			// At this point in time the solution is to retry a few times as it does not fail to
			// retrieve when retried.
			//
			// GitHUB issue - https://github.com/vmware/go-vcloud-director/issues/218
			if err != nil {
				util.Logger.Printf("[DEBUG] vCD 9.7 is known to sometimes respond with error on edge gateway (%s) "+
					"retrieval. As a workaround this is done a few times before failing. Retrying: ", edgegateway)
				for i := 1; i < 4 && err != nil; i++ {
					time.Sleep(200 * time.Millisecond)
					util.Logger.Printf("%d ", i)
					_, err = vdc.client.ExecuteRequest(ctx, href, http.MethodGet,
						"", "error retrieving edge gateway: %s", nil, edge.EdgeGateway)
				}
				util.Logger.Printf("\n")
			}

			return *edge, err

		}
	}
	return EdgeGateway{}, fmt.Errorf("can't find Edge Gateway")

}

// GetEdgeGatewayByHref retrieves an edge gateway from VDC
// by querying directly its HREF.
// The name passed as parameter is only used for error reporting
func (vdc *Vdc) GetEdgeGatewayByHref(ctx context.Context, href string) (*EdgeGateway, error) {
	if href == "" {
		return nil, fmt.Errorf("empty edge gateway HREF")
	}

	edge := NewEdgeGateway(vdc.client)

	_, err := vdc.client.ExecuteRequest(ctx, href, http.MethodGet,
		"", "error retrieving edge gateway: %s", nil, edge.EdgeGateway)

	// TODO - remove this if a solution is found or once 9.7 is deprecated
	// vCD 9.7 has a bug and sometimes it fails to retrieve edge gateway with weird error.
	// At this point in time the solution is to retry a few times as it does not fail to
	// retrieve when retried.
	//
	// GitHUB issue - https://github.com/vmware/go-vcloud-director/issues/218
	if err != nil {
		util.Logger.Printf("[DEBUG] vCD 9.7 is known to sometimes respond with error on edge gateway " +
			"retrieval. As a workaround this is done a few times before failing. Retrying:")
		for i := 1; i < 4 && err != nil; i++ {
			time.Sleep(200 * time.Millisecond)
			util.Logger.Printf("%d ", i)
			_, err = vdc.client.ExecuteRequest(ctx, href, http.MethodGet,
				"", "error retrieving edge gateway: %s", nil, edge.EdgeGateway)
		}
		util.Logger.Printf("\n")
	}

	if err != nil {
		return nil, err
	}
	return edge, nil
}

// GetEdgeGatewayRecordsType retrieves a list of edge gateways from VDC
func (vdc *Vdc) GetEdgeGatewayRecordsType(ctx context.Context, refresh bool) (*types.QueryResultEdgeGatewayRecordsType, error) {

	if refresh {
		err := vdc.Refresh(ctx)
		if err != nil {
			return nil, fmt.Errorf("error refreshing vdc: %s", err)
		}
	}
	for _, av := range vdc.Vdc.Link {
		if av.Rel == "edgeGateways" && av.Type == types.MimeQueryRecords {

			edgeGatewayRecordsType := new(types.QueryResultEdgeGatewayRecordsType)

			_, err := vdc.client.ExecuteRequest(ctx, av.HREF, http.MethodGet,
				"", "error querying edge gateways: %s", nil, edgeGatewayRecordsType)
			if err != nil {
				return nil, err
			}
			return edgeGatewayRecordsType, nil
		}
	}
	return nil, fmt.Errorf("no edge gateway query link found in VDC %s", vdc.Vdc.Name)
}

// GetEdgeGatewayByName search the VDC list of edge gateways for a given name.
// If the name matches, it returns a pointer to an edge gateway object.
// On failure, it returns a nil object and an error
func (vdc *Vdc) GetEdgeGatewayByName(ctx context.Context, name string, refresh bool) (*EdgeGateway, error) {
	edgeGatewayRecord, err := vdc.GetEdgeGatewayRecordsType(ctx, refresh)
	if err != nil {
		return nil, fmt.Errorf("error retrieving edge gateways list: %s", err)
	}

	for _, edge := range edgeGatewayRecord.EdgeGatewayRecord {
		if edge.Name == name {
			return vdc.GetEdgeGatewayByHref(ctx, edge.HREF)
		}
	}

	return nil, ErrorEntityNotFound
}

// GetEdgeGatewayById search VDC list of edge gateways for a given ID.
// If the id matches, it returns a pointer to an edge gateway object.
// On failure, it returns a nil object and an error
func (vdc *Vdc) GetEdgeGatewayById(ctx context.Context, id string, refresh bool) (*EdgeGateway, error) {
	edgeGatewayRecord, err := vdc.GetEdgeGatewayRecordsType(ctx, refresh)
	if err != nil {
		return nil, fmt.Errorf("error retrieving edge gateways list: %s", err)
	}

	for _, edge := range edgeGatewayRecord.EdgeGatewayRecord {
		if equalIds(id, "", edge.HREF) {
			return vdc.GetEdgeGatewayByHref(ctx, edge.HREF)
		}
	}

	return nil, ErrorEntityNotFound
}

// GetEdgeGatewayByNameOrId search the VDC list of edge gateways for a given name or ID.
// If the name or the ID match, it returns a pointer to an edge gateway object.
// On failure, it returns a nil object and an error
func (vdc *Vdc) GetEdgeGatewayByNameOrId(ctx context.Context, identifier string, refresh bool) (*EdgeGateway, error) {
	getByName := func(name string, refresh bool) (interface{}, error) {
		return vdc.GetEdgeGatewayByName(ctx, name, refresh)
	}
	getById := func(id string, refresh bool) (interface{}, error) { return vdc.GetEdgeGatewayById(ctx, id, refresh) }
	entity, err := getEntityByNameOrId(getByName, getById, identifier, false)
	if entity == nil {
		return nil, err
	}
	return entity.(*EdgeGateway), err
}

func (vdc *Vdc) ComposeRawVApp(ctx context.Context, name string) error {
	vcomp := &types.ComposeVAppParams{
		Ovf:     types.XMLNamespaceOVF,
		Xsi:     types.XMLNamespaceXSI,
		Xmlns:   types.XMLNamespaceVCloud,
		Deploy:  false,
		Name:    name,
		PowerOn: false,
	}

	vdcHref, err := url.ParseRequestURI(vdc.Vdc.HREF)
	if err != nil {
		return fmt.Errorf("error getting vdc href: %s", err)
	}
	vdcHref.Path += "/action/composeVApp"

	task, err := vdc.client.ExecuteTaskRequest(ctx, vdcHref.String(), http.MethodPost,
		types.MimeComposeVappParams, "error instantiating a new vApp:: %s", vcomp)
	if err != nil {
		return fmt.Errorf("error executing task request: %s", err)
	}

	err = task.WaitTaskCompletion(ctx)
	if err != nil {
		return fmt.Errorf("error performing task: %s", err)
	}

	return nil
}

// ComposeVApp creates a vapp with the given template, name, and description
// that uses the storageprofile and networks given. If you want all eulas
// to be accepted set acceptalleulas to true. Returns a successful task
// if completed successfully, otherwise returns an error and an empty task.
func (vdc *Vdc) ComposeVApp(ctx context.Context, orgvdcnetworks []*types.OrgVDCNetwork, vapptemplate VAppTemplate, storageprofileref types.Reference, name string, description string, acceptalleulas bool) (Task, error) {
	if vapptemplate.VAppTemplate.Children == nil || orgvdcnetworks == nil {
		return Task{}, fmt.Errorf("can't compose a new vApp, objects passed are not valid")
	}

	// Determine primary network connection index number. We normally depend on it being inherited from vApp template
	// but in the case when vApp template does not have network card it would fail on the index being undefined. We
	// set the value to 0 (first NIC instead)
	primaryNetworkConnectionIndex := 0
	if vapptemplate.VAppTemplate.Children != nil && len(vapptemplate.VAppTemplate.Children.VM) > 0 &&
		vapptemplate.VAppTemplate.Children.VM[0].NetworkConnectionSection != nil {
		primaryNetworkConnectionIndex = vapptemplate.VAppTemplate.Children.VM[0].NetworkConnectionSection.PrimaryNetworkConnectionIndex
	}

	// Build request XML
	vcomp := &types.ComposeVAppParams{
		Ovf:         types.XMLNamespaceOVF,
		Xsi:         types.XMLNamespaceXSI,
		Xmlns:       types.XMLNamespaceVCloud,
		Deploy:      false,
		Name:        name,
		PowerOn:     false,
		Description: description,
		InstantiationParams: &types.InstantiationParams{
			NetworkConfigSection: &types.NetworkConfigSection{
				Info: "Configuration parameters for logical networks",
			},
		},
		AllEULAsAccepted: acceptalleulas,
		SourcedItem: &types.SourcedCompositionItemParam{
			Source: &types.Reference{
				HREF: vapptemplate.VAppTemplate.Children.VM[0].HREF,
				Name: vapptemplate.VAppTemplate.Children.VM[0].Name,
			},
			InstantiationParams: &types.InstantiationParams{
				NetworkConnectionSection: &types.NetworkConnectionSection{
					Info:                          "Network config for sourced item",
					PrimaryNetworkConnectionIndex: primaryNetworkConnectionIndex,
				},
			},
		},
	}
	for index, orgvdcnetwork := range orgvdcnetworks {
		vcomp.InstantiationParams.NetworkConfigSection.NetworkConfig = append(vcomp.InstantiationParams.NetworkConfigSection.NetworkConfig,
			types.VAppNetworkConfiguration{
				NetworkName: orgvdcnetwork.Name,
				Configuration: &types.NetworkConfiguration{
					FenceMode: types.FenceModeBridged,
					ParentNetwork: &types.Reference{
						HREF: orgvdcnetwork.HREF,
						Name: orgvdcnetwork.Name,
						Type: orgvdcnetwork.Type,
					},
				},
			},
		)
		vcomp.SourcedItem.InstantiationParams.NetworkConnectionSection.NetworkConnection = append(vcomp.SourcedItem.InstantiationParams.NetworkConnectionSection.NetworkConnection,
			&types.NetworkConnection{
				Network:                 orgvdcnetwork.Name,
				NetworkConnectionIndex:  index,
				IsConnected:             true,
				IPAddressAllocationMode: types.IPAllocationModePool,
			},
		)
		vcomp.SourcedItem.NetworkAssignment = append(vcomp.SourcedItem.NetworkAssignment,
			&types.NetworkAssignment{
				InnerNetwork:     orgvdcnetwork.Name,
				ContainerNetwork: orgvdcnetwork.Name,
			},
		)
	}
	if storageprofileref.HREF != "" {
		vcomp.SourcedItem.StorageProfile = &storageprofileref
	}

	vdcHref, err := url.ParseRequestURI(vdc.Vdc.HREF)
	if err != nil {
		return Task{}, fmt.Errorf("error getting vdc href: %s", err)
	}
	vdcHref.Path += "/action/composeVApp"

	return vdc.client.ExecuteTaskRequest(ctx, vdcHref.String(), http.MethodPost,
		types.MimeComposeVappParams, "error instantiating a new vApp: %s", vcomp)
}

// Deprecated: use vdc.GetVAppByName instead
func (vdc *Vdc) FindVAppByName(ctx context.Context, vapp string) (VApp, error) {

	err := vdc.Refresh(ctx)
	if err != nil {
		return VApp{}, fmt.Errorf("error refreshing vdc: %s", err)
	}

	for _, resents := range vdc.Vdc.ResourceEntities {
		for _, resent := range resents.ResourceEntity {

			if resent.Name == vapp && resent.Type == "application/vnd.vmware.vcloud.vApp+xml" {

				newVapp := NewVApp(vdc.client)

				_, err := vdc.client.ExecuteRequest(ctx, resent.HREF, http.MethodGet,
					"", "error retrieving vApp: %s", nil, newVapp.VApp)

				return *newVapp, err

			}
		}
	}
	return VApp{}, fmt.Errorf("can't find vApp: %s", vapp)
}

// Deprecated: use vapp.GetVMByName instead
func (vdc *Vdc) FindVMByName(ctx context.Context, vapp VApp, vm string) (VM, error) {

	err := vdc.Refresh(ctx)
	if err != nil {
		return VM{}, fmt.Errorf("error refreshing vdc: %s", err)
	}

	err = vapp.Refresh(ctx)
	if err != nil {
		return VM{}, fmt.Errorf("error refreshing vApp: %s", err)
	}

	//vApp Might Not Have Any VMs

	if vapp.VApp.Children == nil {
		return VM{}, fmt.Errorf("VApp Has No VMs")
	}

	util.Logger.Printf("[TRACE] Looking for VM: %s", vm)
	for _, child := range vapp.VApp.Children.VM {

		util.Logger.Printf("[TRACE] Found: %s", child.Name)
		if child.Name == vm {

			newVm := NewVM(vdc.client)

			_, err := vdc.client.ExecuteRequest(ctx, child.HREF, http.MethodGet,
				"", "error retrieving vm: %s", nil, newVm.VM)

			return *newVm, err
		}

	}
	util.Logger.Printf("[TRACE] Couldn't find VM: %s", vm)
	return VM{}, fmt.Errorf("can't find vm: %s", vm)
}

// Find vm using vApp name and VM name. Returns VMRecord query return type
func (vdc *Vdc) QueryVM(ctx context.Context, vappName, vmName string) (VMRecord, error) {

	if vmName == "" {
		return VMRecord{}, errors.New("error querying vm name is empty")
	}

	if vappName == "" {
		return VMRecord{}, errors.New("error querying vapp name is empty")
	}

	typeMedia := "vm"
	if vdc.client.IsSysAdmin {
		typeMedia = "adminVM"
	}

	results, err := vdc.QueryWithNotEncodedParams(ctx, nil, map[string]string{"type": typeMedia,
		"filter":        "name==" + url.QueryEscape(vmName) + ";containerName==" + url.QueryEscape(vappName),
		"filterEncoded": "true"})
	if err != nil {
		return VMRecord{}, fmt.Errorf("error querying vm %s", err)
	}

	vmResults := results.Results.VMRecord
	if vdc.client.IsSysAdmin {
		vmResults = results.Results.AdminVMRecord
	}

	newVM := NewVMRecord(vdc.client)

	if len(vmResults) == 1 {
		newVM.VM = vmResults[0]
	} else {
		return VMRecord{}, fmt.Errorf("found results %d", len(vmResults))
	}

	return *newVM, nil
}

// Deprecated: use vdc.GetVAppById instead
func (vdc *Vdc) FindVAppByID(ctx context.Context, vappid string) (VApp, error) {

	// Horrible hack to fetch a vapp with its id.
	// urn:vcloud:vapp:00000000-0000-0000-0000-000000000000

	err := vdc.Refresh(ctx)
	if err != nil {
		return VApp{}, fmt.Errorf("error refreshing vdc: %s", err)
	}

	urnslice := strings.SplitAfter(vappid, ":")
	urnid := urnslice[len(urnslice)-1]

	for _, resents := range vdc.Vdc.ResourceEntities {
		for _, resent := range resents.ResourceEntity {

			hrefslice := strings.SplitAfter(resent.HREF, "/")
			hrefslice = strings.SplitAfter(hrefslice[len(hrefslice)-1], "-")
			res := strings.Join(hrefslice[1:], "")

			if res == urnid && resent.Type == "application/vnd.vmware.vcloud.vApp+xml" {

				newVapp := NewVApp(vdc.client)

				_, err := vdc.client.ExecuteRequest(ctx, resent.HREF, http.MethodGet,
					"", "error retrieving vApp: %s", nil, newVapp.VApp)

				return *newVapp, err

			}
		}
	}
	return VApp{}, fmt.Errorf("can't find vApp")

}

// FindMediaImage returns media image found in system using `name` as query.
// Can find a few of them if media with same name exist in different catalogs.
// Deprecated: Use catalog.GetMediaByName()
func (vdc *Vdc) FindMediaImage(ctx context.Context, mediaName string) (MediaItem, error) {
	util.Logger.Printf("[TRACE] Querying medias by name\n")

	mediaResults, err := queryMediaWithFilter(ctx, vdc,
		fmt.Sprintf("name==%s", url.QueryEscape(mediaName)))
	if err != nil {
		return MediaItem{}, err
	}

	newMediaItem := NewMediaItem(vdc)

	if len(mediaResults) == 1 {
		newMediaItem.MediaItem = mediaResults[0]
	}

	if len(mediaResults) == 0 {
		return MediaItem{}, nil
	}

	if len(mediaResults) > 1 {
		return MediaItem{}, errors.New("found more than result")
	}

	util.Logger.Printf("[TRACE] Found media record by name: %#v \n", mediaResults[0])
	return *newMediaItem, nil
}

// GetVappByHref returns a vApp reference by running a vCD API call
// If no valid vApp is found, it returns a nil VApp reference and an error
func (vdc *Vdc) GetVAppByHref(ctx context.Context, vappHref string) (*VApp, error) {

	newVapp := NewVApp(vdc.client)

	_, err := vdc.client.ExecuteRequest(ctx, vappHref, http.MethodGet,
		"", "error retrieving vApp: %s", nil, newVapp.VApp)

	if err != nil {
		return nil, err
	}
	return newVapp, nil
}

// GetVappByName returns a vApp reference if the vApp Name matches an existing one.
// If no valid vApp is found, it returns a nil VApp reference and an error
func (vdc *Vdc) GetVAppByName(ctx context.Context, vappName string, refresh bool) (*VApp, error) {

	if refresh {
		err := vdc.Refresh(ctx)
		if err != nil {
			return nil, fmt.Errorf("error refreshing VDC: %s", err)
		}
	}

	for _, resourceEntities := range vdc.Vdc.ResourceEntities {
		for _, resourceReference := range resourceEntities.ResourceEntity {
			if resourceReference.Name == vappName && resourceReference.Type == "application/vnd.vmware.vcloud.vApp+xml" {
				return vdc.GetVAppByHref(ctx, resourceReference.HREF)
			}
		}
	}
	return nil, ErrorEntityNotFound
}

// GetVappById returns a vApp reference if the vApp ID matches an existing one.
// If no valid vApp is found, it returns a nil VApp reference and an error
func (vdc *Vdc) GetVAppById(ctx context.Context, id string, refresh bool) (*VApp, error) {

	if refresh {
		err := vdc.Refresh(ctx)
		if err != nil {
			return nil, fmt.Errorf("error refreshing VDC: %s", err)
		}
	}

	for _, resourceEntities := range vdc.Vdc.ResourceEntities {
		for _, resourceReference := range resourceEntities.ResourceEntity {
			if equalIds(id, resourceReference.ID, resourceReference.HREF) {
				return vdc.GetVAppByHref(ctx, resourceReference.HREF)
			}
		}
	}
	return nil, ErrorEntityNotFound
}

// GetVappByNameOrId returns a vApp reference if either the vApp name or ID matches an existing one.
// If no valid vApp is found, it returns a nil VApp reference and an error
func (vdc *Vdc) GetVAppByNameOrId(ctx context.Context, identifier string, refresh bool) (*VApp, error) {
	getByName := func(name string, refresh bool) (interface{}, error) { return vdc.GetVAppByName(ctx, name, refresh) }
	getById := func(id string, refresh bool) (interface{}, error) { return vdc.GetVAppById(ctx, id, refresh) }
	entity, err := getEntityByNameOrId(getByName, getById, identifier, false)
	if entity == nil {
		return nil, err
	}
	return entity.(*VApp), err
}

// buildNsxvNetworkServiceEndpointURL uses vDC HREF as a base to derive NSX-V based "network
// services" endpoint (eg: https://_hostname_or_ip_/network/services + optionalSuffix)
func (vdc *Vdc) buildNsxvNetworkServiceEndpointURL(optionalSuffix string) (string, error) {
	apiEndpoint, err := url.ParseRequestURI(vdc.Vdc.HREF)
	if err != nil {
		return "", fmt.Errorf("unable to process vDC URL: %s", err)
	}

	hostname := apiEndpoint.Scheme + "://" + apiEndpoint.Host + "/network/services"

	if optionalSuffix != "" {
		return hostname + optionalSuffix, nil
	}

	return hostname, nil
}

// QueryMediaList retrieves a list of media items for the VDC
func (vdc *Vdc) QueryMediaList(ctx context.Context) ([]*types.MediaRecordType, error) {
	return getExistingMedia(ctx, vdc)
}

// QueryVappVmTemplate Finds VM template using catalog name, vApp template name, VN name in template. Returns types.QueryResultVMRecordType
func (vdc *Vdc) QueryVappVmTemplate(ctx context.Context, catalogName, vappTemplateName, vmNameInTemplate string) (*types.QueryResultVMRecordType, error) {

	queryType := "vm"
	if vdc.client.IsSysAdmin {
		queryType = "adminVM"
	}

	// this allows to query deployed and not deployed templates
	results, err := vdc.QueryWithNotEncodedParams(ctx, nil, map[string]string{"type": queryType,
		"filter": "catalogName==" + url.QueryEscape(catalogName) + ";containerName==" + url.QueryEscape(vappTemplateName) + ";name==" + url.QueryEscape(vmNameInTemplate) +
			";isVAppTemplate==true;status!=FAILED_CREATION;status!=UNKNOWN;status!=UNRECOGNIZED;status!=UNRESOLVED&links=true;",
		"filterEncoded": "true"})
	if err != nil {
		return nil, fmt.Errorf("error quering all vApp templates: %s", err)
	}

	vmResults := results.Results.VMRecord
	if vdc.client.IsSysAdmin {
		vmResults = results.Results.AdminVMRecord
	}

	if len(vmResults) == 0 {
		return nil, fmt.Errorf("[QueryVappVmTemplate] did not find any result with catalog name: %s, "+
			"vApp template name: %s, VM name: %s", catalogName, vappTemplateName, vmNameInTemplate)
	}

	if len(vmResults) > 1 {
		return nil, fmt.Errorf("[QueryVappVmTemplate] found more than 1 result: %d with with catalog name: %s, "+
			"vApp template name: %s, VM name: %s", len(vmResults), catalogName, vappTemplateName, vmNameInTemplate)
	}

	return vmResults[0], nil
}

// getLinkHref returns a link HREF for a wanted combination of rel and type
func (vdc *Vdc) getLinkHref(rel, linkType string) string {
	for _, link := range vdc.Vdc.Link {
		if link.Rel == rel && link.Type == linkType {
			return link.HREF
		}
	}
	return ""
}

// GetVappList returns the list of vApps for a VDC
func (vdc *Vdc) GetVappList() []*types.ResourceReference {
	var list []*types.ResourceReference
	for _, resourceEntities := range vdc.Vdc.ResourceEntities {
		for _, resourceReference := range resourceEntities.ResourceEntity {
			if resourceReference.Type == types.MimeVApp {
				list = append(list, resourceReference)
			}
		}
	}
	return list
}

// CreateStandaloneVmAsync starts a standalone VM creation without a template, returning a task
func (vdc *Vdc) CreateStandaloneVmAsync(ctx context.Context, params *types.CreateVmParams) (Task, error) {
	util.Logger.Printf("[TRACE] Vdc.CreateStandaloneVmAsync - Creating VM ")

	if vdc.Vdc.HREF == "" {
		return Task{}, fmt.Errorf("cannot create VM, Object VDC is empty")
	}

	href := ""
	for _, link := range vdc.Vdc.Link {
		if link.Type == types.MimeCreateVmParams && link.Rel == "add" {
			href = link.HREF
			break
		}
	}
	if href == "" {
		return Task{}, fmt.Errorf("error retrieving VM creation link from VDC %s", vdc.Vdc.Name)
	}
	if params == nil {
		return Task{}, fmt.Errorf("empty parameters passed to standalone VM creation")
	}
	params.XmlnsOvf = types.XMLNamespaceOVF

	return vdc.client.ExecuteTaskRequest(ctx, href, http.MethodPost, types.MimeCreateVmParams, "error creating standalone VM: %s", params)
}

// getVmFromTask finds a VM from a running standalone VM creation task
// It retrieves the VM owner (the hidden vApp), and from that one finds the new VM
func (vdc *Vdc) getVmFromTask(ctx context.Context, task Task, name string) (*VM, error) {
	owner := task.Task.Owner.HREF
	if owner == "" {
		return nil, fmt.Errorf("task owner is null for VM %s", name)
	}
	vapp, err := vdc.GetVAppByHref(ctx, owner)
	if err != nil {
		return nil, err
	}
	if vapp.VApp.Children == nil {
		return nil, ErrorEntityNotFound
	}
	if len(vapp.VApp.Children.VM) == 0 {
		return nil, fmt.Errorf("vApp %s contains no VMs", vapp.VApp.Name)
	}
	if len(vapp.VApp.Children.VM) > 1 {
		return nil, fmt.Errorf("vApp %s contains more than one VM", vapp.VApp.Name)
	}
	for _, child := range vapp.VApp.Children.VM {
		util.Logger.Printf("[TRACE] Looking at: %s", child.Name)
		return vapp.client.GetVMByHref(ctx, child.HREF)
	}
	return nil, ErrorEntityNotFound
}

// CreateStandaloneVm creates a standalone VM without a template
func (vdc *Vdc) CreateStandaloneVm(ctx context.Context, params *types.CreateVmParams) (*VM, error) {

	task, err := vdc.CreateStandaloneVmAsync(ctx, params)
	if err != nil {
		return nil, err
	}
	err = task.WaitTaskCompletion(ctx)
	if err != nil {
		return nil, err
	}
	return vdc.getVmFromTask(ctx, task, params.Name)
}

// QueryVmByName finds a standalone VM by name
// The search fails either if there are more VMs with the wanted name, or if there are none
// It can also retrieve a standard VM (created from vApp)
func (vdc *Vdc) QueryVmByName(ctx context.Context, name string) (*VM, error) {
	vmList, err := vdc.QueryVmList(ctx, types.VmQueryFilterOnlyDeployed)
	if err != nil {
		return nil, err
	}
	var foundVM []*types.QueryResultVMRecordType
	for _, vm := range vmList {
		if vm.Name == name {
			foundVM = append(foundVM, vm)
		}
	}
	if len(foundVM) == 0 {
		return nil, ErrorEntityNotFound
	}
	if len(foundVM) > 1 {
		return nil, fmt.Errorf("more than one VM found with name %s", name)
	}
	return vdc.client.GetVMByHref(ctx, foundVM[0].HREF)
}

// QueryVmById retrieves a standalone VM by ID
// It can also retrieve a standard VM (created from vApp)
func (vdc *Vdc) QueryVmById(ctx context.Context, id string) (*VM, error) {
	vmList, err := vdc.QueryVmList(ctx, types.VmQueryFilterOnlyDeployed)
	if err != nil {
		return nil, err
	}
	var foundVM []*types.QueryResultVMRecordType
	for _, vm := range vmList {
		if equalIds(id, vm.ID, vm.HREF) {
			foundVM = append(foundVM, vm)
		}
	}
	if len(foundVM) == 0 {
		return nil, ErrorEntityNotFound
	}
	if len(foundVM) > 1 {
		return nil, fmt.Errorf("more than one VM found with ID %s", id)
	}
	return vdc.client.GetVMByHref(ctx, foundVM[0].HREF)
}

// CreateStandaloneVMFromTemplateAsync starts a standalone VM creation using a template
func (vdc *Vdc) CreateStandaloneVMFromTemplateAsync(ctx context.Context, params *types.InstantiateVmTemplateParams) (Task, error) {

	util.Logger.Printf("[TRACE] Vdc.CreateStandaloneVMFromTemplateAsync - Creating VM")

	if vdc.Vdc.HREF == "" {
		return Task{}, fmt.Errorf("cannot create VM, provided VDC is empty")
	}

	href := ""
	for _, link := range vdc.Vdc.Link {
		if link.Type == types.MimeInstantiateVmTemplateParams && link.Rel == "add" {
			href = link.HREF
			break
		}
	}
	if href == "" {
		return Task{}, fmt.Errorf("error retrieving VM instantiate from template link from VDC %s", vdc.Vdc.Name)
	}

	if params.Name == "" {
		return Task{}, fmt.Errorf("[CreateStandaloneVMFromTemplateAsync] missing VM name")
	}
	if params.SourcedVmTemplateItem == nil {
		return Task{}, fmt.Errorf("[CreateStandaloneVMFromTemplateAsync] missing SourcedVmTemplateItem")
	}
	if params.SourcedVmTemplateItem.Source == nil {
		return Task{}, fmt.Errorf("[CreateStandaloneVMFromTemplateAsync] missing vApp template Source")
	}
	if params.SourcedVmTemplateItem.Source.HREF == "" {
		return Task{}, fmt.Errorf("[CreateStandaloneVMFromTemplateAsync] empty HREF in vApp template Source")
	}
	params.XmlnsOvf = types.XMLNamespaceOVF

	return vdc.client.ExecuteTaskRequest(ctx, href, http.MethodPost, types.MimeInstantiateVmTemplateParams, "error creating standalone VM from template: %s", params)
}

// CreateStandaloneVMFromTemplate creates a standalone VM from a template
func (vdc *Vdc) CreateStandaloneVMFromTemplate(ctx context.Context, params *types.InstantiateVmTemplateParams) (*VM, error) {

	task, err := vdc.CreateStandaloneVMFromTemplateAsync(ctx, params)
	if err != nil {
		return nil, err
	}
	err = task.WaitTaskCompletion(ctx)
	if err != nil {
		return nil, err
	}
	return vdc.getVmFromTask(ctx, task, params.Name)
}

// GetCapabilities allows to retrieve a list of VDC capabilities. It has a list of values. Some particularly useful are:
// * networkProvider - overlay stack responsible for providing network functionality. (NSX_V or NSX_T)
// * crossVdc - supports cross vDC network creation
func (vdc *Vdc) GetCapabilities(ctx context.Context) ([]types.VdcCapability, error) {
	if vdc.Vdc.ID == "" {
		return nil, fmt.Errorf("VDC ID must be set to get capabilities")
	}

	endpoint := types.OpenApiPathVersion1_0_0 + types.OpenApiEndpointVdcCapabilities
	minimumApiVersion, err := vdc.client.checkOpenApiEndpointCompatibility(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	urlRef, err := vdc.client.OpenApiBuildEndpoint(fmt.Sprintf(endpoint, url.QueryEscape(vdc.Vdc.ID)))
	if err != nil {
		return nil, err
	}

	capabilities := make([]types.VdcCapability, 0)
	err = vdc.client.OpenApiGetAllItems(ctx, minimumApiVersion, urlRef, nil, &capabilities)
	if err != nil {
		return nil, err
	}
	return capabilities, nil
}

// IsNsxt is a convenience function to check if VDC is backed by NSX-T pVdc
// If error occurs - it returns false
func (vdc *Vdc) IsNsxt(ctx context.Context) bool {
	vdcCapabilities, err := vdc.GetCapabilities(ctx)
	if err != nil {
		return false
	}

	networkProviderCapability := getCapabilityValue(vdcCapabilities, "networkProvider")
	return networkProviderCapability == types.VdcCapabilityNetworkProviderNsxt
}

// IsNsxv is a convenience function to check if VDC is backed by NSX-V pVdc
// If error occurs - it returns false
func (vdc *Vdc) IsNsxv(ctx context.Context) bool {
	vdcCapabilities, err := vdc.GetCapabilities(ctx)
	if err != nil {
		return false
	}

	networkProviderCapability := getCapabilityValue(vdcCapabilities, "networkProvider")
	return networkProviderCapability == types.VdcCapabilityNetworkProviderNsxv
}

// getCapabilityValue helps to lookup a specific capability in []types.VdcCapability by provided fieldName
func getCapabilityValue(capabilities []types.VdcCapability, fieldName string) string {
	for _, field := range capabilities {
		if field.Name == fieldName {
			return field.Value.(string)
		}
	}

	return ""
}
