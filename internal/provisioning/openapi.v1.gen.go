// Package provisioning provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.12.4 DO NOT EDIT.
package provisioning

import (
	"time"
)

// Defines values for GetSourceListParamsProvider.
const (
	Aws   GetSourceListParamsProvider = "aws"
	Azure GetSourceListParamsProvider = "azure"
	Gcp   GetSourceListParamsProvider = "gcp"
)

// V1AWSReservationRequest defines model for v1.AWSReservationRequest.
type V1AWSReservationRequest struct {
	Amount           *int32  `json:"amount,omitempty"`
	ImageId          *string `json:"image_id,omitempty"`
	InstanceType     *string `json:"instance_type,omitempty"`
	LaunchTemplateId *string `json:"launch_template_id,omitempty"`
	Name             *string `json:"name,omitempty"`
	Poweroff         *bool   `json:"poweroff,omitempty"`
	PubkeyId         *int64  `json:"pubkey_id,omitempty"`
	Region           *string `json:"region,omitempty"`
	SourceId         *string `json:"source_id,omitempty"`
}

// V1AWSReservationResponse defines model for v1.AWSReservationResponse.
type V1AWSReservationResponse struct {
	Amount           *int32  `json:"amount,omitempty"`
	AwsReservationId *string `json:"aws_reservation_id,omitempty"`
	ImageId          *string `json:"image_id,omitempty"`
	InstanceType     *string `json:"instance_type,omitempty"`
	Instances        *[]struct {
		Detail *struct {
			PrivateIpv4 *string `json:"private_ipv4,omitempty"`
			PrivateIpv6 *string `json:"private_ipv6,omitempty"`
			PublicDns   *string `json:"public_dns,omitempty"`
			PublicIpv4  *string `json:"public_ipv4,omitempty"`
		} `json:"detail,omitempty"`
		InstanceId *string `json:"instance_id,omitempty"`
	} `json:"instances,omitempty"`
	LaunchTemplateId *string `json:"launch_template_id,omitempty"`
	Name             *string `json:"name,omitempty"`
	Poweroff         *bool   `json:"poweroff,omitempty"`
	PubkeyId         *int64  `json:"pubkey_id,omitempty"`
	Region           *string `json:"region,omitempty"`
	ReservationId    *int64  `json:"reservation_id,omitempty"`
	SourceId         *string `json:"source_id,omitempty"`
}

// V1AvailabilityStatusRequest defines model for v1.AvailabilityStatusRequest.
type V1AvailabilityStatusRequest struct {
	SourceId *string `json:"source_id,omitempty"`
}

// V1AzureReservationRequest defines model for v1.AzureReservationRequest.
type V1AzureReservationRequest struct {
	Amount       *int64  `json:"amount,omitempty"`
	ImageId      *string `json:"image_id,omitempty"`
	InstanceSize *string `json:"instance_size,omitempty"`

	// Location Location (also known as region) to deploy the VM into, be aware it needs to be the same as the image location. Defaults to the Resource Group location, or 'eastus' when also creating the resource group.
	Location *string `json:"location,omitempty"`

	// Name Name of the instance, to keep names unique, it will be suffixed with UUID. Optional, defaults to 'redhat-vm''
	Name     *string `json:"name,omitempty"`
	Poweroff *bool   `json:"poweroff,omitempty"`
	PubkeyId *int64  `json:"pubkey_id,omitempty"`

	// ResourceGroup Azure resource group name to deploy the VM resources into. Optional, defaults to images resource group and when not found to 'redhat-deployed'.
	ResourceGroup *string `json:"resource_group,omitempty"`
	SourceId      *string `json:"source_id,omitempty"`
}

// V1AzureReservationResponse defines model for v1.AzureReservationResponse.
type V1AzureReservationResponse struct {
	Amount       *int64  `json:"amount,omitempty"`
	ImageId      *string `json:"image_id,omitempty"`
	InstanceSize *string `json:"instance_size,omitempty"`
	Instances    *[]struct {
		Detail *struct {
			PrivateIpv4 *string `json:"private_ipv4,omitempty"`
			PrivateIpv6 *string `json:"private_ipv6,omitempty"`
			PublicDns   *string `json:"public_dns,omitempty"`
			PublicIpv4  *string `json:"public_ipv4,omitempty"`
		} `json:"detail,omitempty"`
		InstanceId *string `json:"instance_id,omitempty"`
	} `json:"instances,omitempty"`
	Location      *string `json:"location,omitempty"`
	Name          *string `json:"name,omitempty"`
	Poweroff      *bool   `json:"poweroff,omitempty"`
	PubkeyId      *int64  `json:"pubkey_id,omitempty"`
	ReservationId *int64  `json:"reservation_id,omitempty"`
	ResourceGroup *string `json:"resource_group,omitempty"`
	SourceId      *string `json:"source_id,omitempty"`
}

// V1GCPReservationRequest defines model for v1.GCPReservationRequest.
type V1GCPReservationRequest struct {
	Amount           *int64  `json:"amount,omitempty"`
	ImageId          *string `json:"image_id,omitempty"`
	LaunchTemplateId *string `json:"launch_template_id,omitempty"`
	MachineType      *string `json:"machine_type,omitempty"`
	NamePattern      *string `json:"name_pattern,omitempty"`
	Poweroff         *bool   `json:"poweroff,omitempty"`
	PubkeyId         *int64  `json:"pubkey_id,omitempty"`
	SourceId         *string `json:"source_id,omitempty"`
	Zone             *string `json:"zone,omitempty"`
}

// V1GCPReservationResponse defines model for v1.GCPReservationResponse.
type V1GCPReservationResponse struct {
	Amount           *int64  `json:"amount,omitempty"`
	GcpOperationName *string `json:"gcp_operation_name,omitempty"`
	ImageId          *string `json:"image_id,omitempty"`
	Instances        *[]struct {
		Detail *struct {
			PrivateIpv4 *string `json:"private_ipv4,omitempty"`
			PrivateIpv6 *string `json:"private_ipv6,omitempty"`
			PublicDns   *string `json:"public_dns,omitempty"`
			PublicIpv4  *string `json:"public_ipv4,omitempty"`
		} `json:"detail,omitempty"`
		InstanceId *string `json:"instance_id,omitempty"`
	} `json:"instances,omitempty"`
	LaunchTemplateId *string `json:"launch_template_id,omitempty"`
	MachineType      *string `json:"machine_type,omitempty"`
	NamePattern      *string `json:"name_pattern,omitempty"`
	Poweroff         *bool   `json:"poweroff,omitempty"`
	PubkeyId         *int64  `json:"pubkey_id,omitempty"`
	ReservationId    *int64  `json:"reservation_id,omitempty"`
	SourceId         *string `json:"source_id,omitempty"`
	Zone             *string `json:"zone,omitempty"`
}

// V1GenericReservationResponse defines model for v1.GenericReservationResponse.
type V1GenericReservationResponse struct {
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	Error      *string    `json:"error,omitempty"`
	FinishedAt *time.Time `json:"finished_at"`
	Id         *int64     `json:"id,omitempty"`
	Provider   *int       `json:"provider,omitempty"`
	Status     *string    `json:"status,omitempty"`
	Step       *int32     `json:"step,omitempty"`
	StepTitles *[]string  `json:"step_titles,omitempty"`
	Steps      *int32     `json:"steps,omitempty"`
	Success    *bool      `json:"success"`
}

// V1ListGenericReservationResponse defines model for v1.ListGenericReservationResponse.
type V1ListGenericReservationResponse struct {
	Data *[]struct {
		CreatedAt  *time.Time `json:"created_at,omitempty"`
		Error      *string    `json:"error,omitempty"`
		FinishedAt *time.Time `json:"finished_at"`
		Id         *int64     `json:"id,omitempty"`
		Provider   *int       `json:"provider,omitempty"`
		Status     *string    `json:"status,omitempty"`
		Step       *int32     `json:"step,omitempty"`
		StepTitles *[]string  `json:"step_titles,omitempty"`
		Steps      *int32     `json:"steps,omitempty"`
		Success    *bool      `json:"success"`
	} `json:"data,omitempty"`
	Metadata *struct {
		Links *struct {
			Next     *string `json:"next,omitempty"`
			Previous *string `json:"previous,omitempty"`
		} `json:"links,omitempty"`
		Total *int `json:"total,omitempty"`
	} `json:"metadata,omitempty"`
}

// V1ListInstaceTypeResponse defines model for v1.ListInstaceTypeResponse.
type V1ListInstaceTypeResponse struct {
	Data *[]struct {
		Architecture *string `json:"architecture,omitempty"`
		Azure        *struct {
			GenV1 *bool `json:"gen_v1,omitempty"`
			GenV2 *bool `json:"gen_v2,omitempty"`
		} `json:"azure,omitempty"`
		Cores     *int32  `json:"cores,omitempty"`
		MemoryMib *int64  `json:"memory_mib,omitempty"`
		Name      *string `json:"name,omitempty"`
		StorageGb *int64  `json:"storage_gb,omitempty"`
		Supported *bool   `json:"supported,omitempty"`
		Vcpus     *int32  `json:"vcpus,omitempty"`
	} `json:"data,omitempty"`
}

// V1ListLaunchTemplateResponse defines model for v1.ListLaunchTemplateResponse.
type V1ListLaunchTemplateResponse struct {
	Data *[]struct {
		Id   *string `json:"id,omitempty"`
		Name *string `json:"name,omitempty"`
	} `json:"data,omitempty"`
	Metadata *struct {
		Links *struct {
			Next     *string `json:"next,omitempty"`
			Previous *string `json:"previous,omitempty"`
		} `json:"links,omitempty"`
		Total *int `json:"total,omitempty"`
	} `json:"metadata,omitempty"`
}

// V1ListPubkeyResponse defines model for v1.ListPubkeyResponse.
type V1ListPubkeyResponse struct {
	Data *[]struct {
		Body              *string `json:"body,omitempty"`
		Fingerprint       *string `json:"fingerprint,omitempty"`
		FingerprintLegacy *string `json:"fingerprint_legacy,omitempty"`
		Id                *int64  `json:"id,omitempty"`
		Name              *string `json:"name,omitempty"`
		Type              *string `json:"type,omitempty"`
	} `json:"data,omitempty"`
	Metadata *struct {
		Links *struct {
			Next     *string `json:"next,omitempty"`
			Previous *string `json:"previous,omitempty"`
		} `json:"links,omitempty"`
		Total *int `json:"total,omitempty"`
	} `json:"metadata,omitempty"`
}

// V1ListSourceResponse defines model for v1.ListSourceResponse.
type V1ListSourceResponse struct {
	Data *[]struct {
		Id   *string `json:"id,omitempty"`
		Name *string `json:"name,omitempty"`

		// Provider One of ('azure', 'aws', 'gcp')
		Provider     *string `json:"provider,omitempty"`
		SourceTypeId *string `json:"source_type_id,omitempty"`
		Status       *string `json:"status,omitempty"`
		Uid          *string `json:"uid,omitempty"`
	} `json:"data,omitempty"`
	Metadata *struct {
		Links *struct {
			Next     *string `json:"next,omitempty"`
			Previous *string `json:"previous,omitempty"`
		} `json:"links,omitempty"`
		Total *int `json:"total,omitempty"`
	} `json:"metadata,omitempty"`
}

// V1NoopReservationResponse defines model for v1.NoopReservationResponse.
type V1NoopReservationResponse struct {
	ReservationId *int64 `json:"reservation_id,omitempty"`
}

// V1PubkeyRequest defines model for v1.PubkeyRequest.
type V1PubkeyRequest struct {
	// Body Add a public part of a SSH key pair.
	Body *string `json:"body,omitempty"`

	// Name Enter the name of the newly created pubkey.
	Name *string `json:"name,omitempty"`
}

// V1PubkeyResponse defines model for v1.PubkeyResponse.
type V1PubkeyResponse struct {
	Body              *string `json:"body,omitempty"`
	Fingerprint       *string `json:"fingerprint,omitempty"`
	FingerprintLegacy *string `json:"fingerprint_legacy,omitempty"`
	Id                *int64  `json:"id,omitempty"`
	Name              *string `json:"name,omitempty"`
	Type              *string `json:"type,omitempty"`
}

// V1ResponseError defines model for v1.ResponseError.
type V1ResponseError struct {
	BuildTime   *string `json:"build_time,omitempty"`
	EdgeId      *string `json:"edge_id,omitempty"`
	Environment *string `json:"environment,omitempty"`
	Error       *string `json:"error,omitempty"`
	Msg         *string `json:"msg,omitempty"`
	TraceId     *string `json:"trace_id,omitempty"`
	Version     *string `json:"version,omitempty"`
}

// V1SourceUploadInfoResponse defines model for v1.SourceUploadInfoResponse.
type V1SourceUploadInfoResponse struct {
	Aws *struct {
		AccountId *string `json:"account_id,omitempty"`
	} `json:"aws"`
	Azure *struct {
		ResourceGroups *[]string `json:"resource_groups,omitempty"`
		SubscriptionId *string   `json:"subscription_id,omitempty"`
		TenantId       *string   `json:"tenant_id,omitempty"`
	} `json:"azure"`
	Gcp      *interface{} `json:"gcp"`
	Provider *string      `json:"provider,omitempty"`
}

// Limit defines model for Limit.
type Limit = int

// Offset defines model for Offset.
type Offset = int

// Token defines model for Token.
type Token = string

// InternalError defines model for InternalError.
type InternalError = V1ResponseError

// NotFound defines model for NotFound.
type NotFound = V1ResponseError

// GetInstanceTypeListAllParams defines parameters for GetInstanceTypeListAll.
type GetInstanceTypeListAllParams struct {
	// Region Region to list instance types within. This is required.
	Region string `form:"region" json:"region"`

	// Zone Availability zone (or location) to list instance types within. Not applicable for AWS EC2 as all zones within a region are the same (will lead to an error when used). Required for Azure.
	Zone *string `form:"zone,omitempty" json:"zone,omitempty"`
}

// GetPubkeyListParams defines parameters for GetPubkeyList.
type GetPubkeyListParams struct {
	// Limit The number of items to return.
	Limit *Limit `form:"limit,omitempty" json:"limit,omitempty"`

	// Offset The number of items to skip before starting to collect the result set.
	Offset *Offset `form:"offset,omitempty" json:"offset,omitempty"`
}

// GetReservationsListParams defines parameters for GetReservationsList.
type GetReservationsListParams struct {
	// Limit The number of items to return.
	Limit *Limit `form:"limit,omitempty" json:"limit,omitempty"`

	// Offset The number of items to skip before starting to collect the result set.
	Offset *Offset `form:"offset,omitempty" json:"offset,omitempty"`
}

// GetSourceListParams defines parameters for GetSourceList.
type GetSourceListParams struct {
	Provider *GetSourceListParamsProvider `form:"provider,omitempty" json:"provider,omitempty"`

	// Limit The number of items to return.
	Limit *Limit `form:"limit,omitempty" json:"limit,omitempty"`

	// Offset The number of items to skip before starting to collect the result set.
	Offset *Offset `form:"offset,omitempty" json:"offset,omitempty"`
}

// GetSourceListParamsProvider defines parameters for GetSourceList.
type GetSourceListParamsProvider string

// GetLaunchTemplatesListParams defines parameters for GetLaunchTemplatesList.
type GetLaunchTemplatesListParams struct {
	// Region Hyperscaler region
	Region string `form:"region" json:"region"`

	// Token The token used for requesting the next page of results; empty token for the first page
	Token *Token `form:"token,omitempty" json:"token,omitempty"`

	// Limit The number of items to return.
	Limit *Limit `form:"limit,omitempty" json:"limit,omitempty"`
}

// AvailabilityStatusJSONRequestBody defines body for AvailabilityStatus for application/json ContentType.
type AvailabilityStatusJSONRequestBody = V1AvailabilityStatusRequest

// CreatePubkeyJSONRequestBody defines body for CreatePubkey for application/json ContentType.
type CreatePubkeyJSONRequestBody = V1PubkeyRequest

// CreateAwsReservationJSONRequestBody defines body for CreateAwsReservation for application/json ContentType.
type CreateAwsReservationJSONRequestBody = V1AWSReservationRequest

// CreateAzureReservationJSONRequestBody defines body for CreateAzureReservation for application/json ContentType.
type CreateAzureReservationJSONRequestBody = V1AzureReservationRequest

// CreateGCPReservationJSONRequestBody defines body for CreateGCPReservation for application/json ContentType.
type CreateGCPReservationJSONRequestBody = V1GCPReservationRequest
