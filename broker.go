package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"

	"github.com/pivotal-cf/brokerapi/v7"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"github.com/teris-io/shortid"
	"toolman.org/numbers/stdsize"
)

type broker struct {
	services    []brokerapi.Service
	env         brokerConfig
	ontapClient *OntapClient
}

type ProvisionParameters struct {
	Size string `json:"size"`
}

func (b *broker) Services(context context.Context) ([]brokerapi.Service, error) {
	return b.services, nil
}

func (b *broker) Provision(context context.Context, instanceID string, details domain.ProvisionDetails, asyncAllowed bool) (domain.ProvisionedServiceSpec, error) {
	if !asyncAllowed {
		return domain.ProvisionedServiceSpec{}, apiresponses.ErrAsyncRequired
	}

	//generate the name
	volumeName := generateVolumeName(instanceID)

	//get params (sieze only for now)
	if details.RawParameters == nil || len(details.RawParameters) == 0 {
		return domain.ProvisionedServiceSpec{}, apiresponses.ErrRawParamsInvalid
	}

	var params ProvisionParameters
	err := json.Unmarshal(details.RawParameters, &params)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, apiresponses.ErrRawParamsInvalid
	}

	size, err := stdsize.Parse(params.Size)
	if err != nil || size < 20971520 {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("Requested volume is smaller than smallest allowed volume")
	}

	if size > stdsize.Value(b.env.MaxVolumeSizeBytes) {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("Requested volume size exceeds configured maximum volume size. You requested %s, Max is %s", params.Size, b.env.MaxVolumeSize)
	}

	jobID, err := b.ontapClient.CreateCifsVolume(volumeName, b.env.OntapSvmName, int64(size))
	if err != nil {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("Create Volume failed: %s", err)
	}

	return domain.ProvisionedServiceSpec{
		IsAsync:       true,
		AlreadyExists: false,
		DashboardURL:  "",
		OperationData: jobID,
	}, nil
}

func (b *broker) GetInstance(ctx context.Context, instanceID string) (domain.GetInstanceDetailsSpec, error) {
	return domain.GetInstanceDetailsSpec{}, fmt.Errorf("Instances are not retrievable")
}

func (b *broker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (domain.DeprovisionServiceSpec, error) {
	if !asyncAllowed {
		return domain.DeprovisionServiceSpec{}, apiresponses.ErrAsyncRequired
	}

	name := generateVolumeName(instanceID)
	id, err := b.ontapClient.GetVolumeIDByName(name)
	if err != nil {
		return domain.DeprovisionServiceSpec{}, fmt.Errorf("error lookup volume with name %s", name)
	}

	jobID, err := b.ontapClient.DeleteVolume(id)
	if err != nil {
		return domain.DeprovisionServiceSpec{}, nil
	}

	return domain.DeprovisionServiceSpec{
		IsAsync:       true,
		OperationData: jobID,
	}, nil
}

func (b *broker) hash(mountOpts map[string]interface{}) (string, error) {
	var (
		bytes []byte
		err   error
	)
	if bytes, err = json.Marshal(mountOpts); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5.Sum(bytes)), nil
}

func (b *broker) Bind(context context.Context, instanceID, bindingID string, details domain.BindDetails, asyncAllowed bool) (domain.Binding, error) {
	volumeName := generateVolumeName(instanceID)

	username, _ := shortid.Generate()
	password, _ := shortid.Generate()
	err := b.ontapClient.CreateCifsUser(username, password, bindingID, b.env.OntapSvmName)
	if err != nil {
		return domain.Binding{}, fmt.Errorf("CreateCifsUser failed: %s", err)
	}

	svmId, err := b.ontapClient.GetSvmIdByName(b.env.OntapSvmName)
	if err != nil {
		return domain.Binding{}, fmt.Errorf("GetSvmIdByName failed: %s", err)
	}

	err = b.ontapClient.AssignCifsUser(username, svmId, volumeName)
	if err != nil {
		return domain.Binding{}, fmt.Errorf("AssignCifsUser failed: %s", err)
	}

	var bindOpts map[string]interface{}
	if len(details.RawParameters) > 0 {
		if err = json.Unmarshal(details.RawParameters, &bindOpts); err != nil {
			return domain.Binding{}, err
		}
	}

	containerPath := fmt.Sprintf("/var/vcap/data/%s", volumeName)
	if path, ok := bindOpts["mount"].(string); ok && path != "" {
		containerPath = path
	}

	mountConfig := make(map[string]interface{})
	mountConfig["version"] = "3.0"
	mountConfig["username"] = username
	mountConfig["password"] = password
	mountConfig["source"] = fmt.Sprintf("//%s/%s", b.env.CifsHostname, volumeName)

	return domain.Binding{
		Credentials: struct{}{}, // if nil, cloud controller chokes on response
		VolumeMounts: []domain.VolumeMount{{
			ContainerDir: containerPath,
			Mode:         "rw",
			Driver:       "smbdriver",
			DeviceType:   "shared",
			Device: domain.SharedDevice{
				VolumeId:    instanceID,
				MountConfig: mountConfig,
			},
		}},
	}, nil
}

func (b *broker) GetBinding(ctx context.Context, instanceID, bindingID string) (domain.GetBindingSpec, error) {
	return domain.GetBindingSpec{}, fmt.Errorf("Bindings are not retrievable")
}

func (b *broker) Unbind(context context.Context, instanceID, bindingID string, details domain.UnbindDetails, asyncAllowed bool) (domain.UnbindSpec, error) {
	user, err := b.ontapClient.GetCifsUserByFullname(b.env.OntapSvmName, bindingID)
	if err != nil {
		return domain.UnbindSpec{}, err
	}

	err = b.ontapClient.DeleteCifsUser(b.env.OntapSvmName, user)
	if err != nil {
		return domain.UnbindSpec{}, err
	}

	return domain.UnbindSpec{}, nil
}

func (b *broker) Update(context context.Context, instanceID string, details domain.UpdateDetails, asyncAllowed bool) (domain.UpdateServiceSpec, error) {

	return domain.UpdateServiceSpec{}, nil
}

func (b *broker) LastOperation(context context.Context, instanceID string, details domain.PollDetails) (domain.LastOperation, error) {
	status, err := b.ontapClient.GetJobStatus(details.OperationData)

	if err != nil {
		fmt.Println(err)
		return domain.LastOperation{
			State:       domain.Failed,
			Description: err.Error(),
		}, fmt.Errorf("Getting status for job %s failed", details.OperationData)
	}

	var jobStatus JobStatus
	json.Unmarshal(status.body, &jobStatus)

	return domain.LastOperation{
		State:       statusMap[jobStatus.State],
		Description: jobStatus.Description,
	}, nil
}

func (b *broker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	return domain.LastOperation{}, nil
}
