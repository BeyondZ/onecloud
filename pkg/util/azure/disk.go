package azure

import (
	"context"
	"fmt"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/compute/models"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"
)

type StorageAccountTypes string

const (
	// StorageAccountTypesPremiumLRS ...
	StorageAccountTypesPremiumLRS StorageAccountTypes = "Premium_LRS"
	// StorageAccountTypesStandardLRS ...
	StorageAccountTypesStandardLRS StorageAccountTypes = "Standard_LRS"
	// StorageAccountTypesStandardSSDLRS ...
	StorageAccountTypesStandardSSDLRS StorageAccountTypes = "StandardSSD_LRS"
)

type DiskSku struct {
	Name StorageAccountTypes
	Tier string
}

type ImageDiskReference struct {
	ID  string
	Lun int32
}

type CreationData struct {
	CreateOption     string
	StorageAccountID string
	ImageReference   ImageDiskReference
	SourceURI        string
	SourceResourceID string
}

type DiskProperties struct {
	TimeCreated       time.Time
	OsType            OperatingSystemTypes
	CreationData      CreationData
	DiskSizeGB        int32
	ProvisioningState string
}

const DiskResourceGroup = "YunionDiskResource"

type SDisk struct {
	storage *SStorage

	ResourceGroup string

	ManagedBy  string
	Sku        DiskSku
	Zones      []string
	ID         string
	Name       string
	Type       string
	Location   string
	Properties DiskProperties

	Tags map[string]string
}

func (self *SRegion) CreateDisk(storageType string, name string, sizeGb int32, desc string) error {
	return self.createDisk(storageType, name, sizeGb, desc)
}

func (self *SRegion) createDisk(storageType string, name string, sizeGb int32, desc string) error {
	computeClient := compute.NewDisksClientWithBaseURI(self.client.baseUrl, self.client.subscriptionId)
	computeClient.Authorizer = self.client.authorizer
	sku := compute.DiskSku{Name: compute.StorageAccountTypes(storageType)}
	properties := compute.DiskProperties{DiskSizeGB: &sizeGb, CreationData: &compute.CreationData{CreateOption: "Empty"}}
	disk := compute.Disk{Name: &name, Location: &self.Name, DiskProperties: &properties, Sku: &sku}
	if result, err := computeClient.CreateOrUpdate(context.Background(), DefaultResourceGroup["disk"], name, disk); err != nil {
		return err
	} else if err := result.WaitForCompletion(context.Background(), computeClient.Client); err != nil {
		return err
	} else {
		return nil
	}
}

func (self *SRegion) DeleteDisk(diskId string) error {
	return self.deleteDisk(diskId)
}

func (self *SRegion) deleteDisk(diskId string) error {
	diskClient := compute.NewDisksClientWithBaseURI(self.client.baseUrl, self.client.subscriptionId)
	diskClient.Authorizer = self.client.authorizer
	if resourceGroup, name, err := PareResourceGroupWithName(diskId); err != nil {
		return err
	} else if result, err := diskClient.Delete(context.Background(), resourceGroup, name); err != nil {
		return err
	} else if err := result.WaitForCompletion(context.Background(), diskClient.Client); err != nil {
		return err
	}
	return nil
}

func (self *SRegion) GetDisk(resourceGroup string, diskName string) (*SDisk, error) {
	disk := SDisk{}
	computeClient := compute.NewDisksClientWithBaseURI(self.client.baseUrl, self.client.subscriptionId)
	computeClient.Authorizer = self.client.authorizer
	if _disk, err := computeClient.Get(context.Background(), resourceGroup, diskName); err != nil {
		return nil, err
	} else if err := jsonutils.Update(&disk, _disk); err != nil {
		return nil, err
	} else {
		return &disk, nil
	}
}

func (self *SRegion) GetDisks() ([]SDisk, error) {
	disks := make([]SDisk, 0)
	computeClient := compute.NewDisksClientWithBaseURI(self.client.baseUrl, self.client.subscriptionId)
	computeClient.Authorizer = self.client.authorizer
	if diskList, err := computeClient.List(context.Background()); err != nil {
		return nil, err
	} else {
		for _, _disk := range diskList.Values() {
			if *_disk.Location == self.Name {
				disk := SDisk{}
				if err := jsonutils.Update(&disk, _disk); err != nil {
					return disks, err
				}
				disk.ResourceGroup, _, _ = PareResourceGroupWithName(disk.ID)
				disks = append(disks, disk)
			}
		}
	}
	return disks, nil
}

func (self *SDisk) GetMetadata() *jsonutils.JSONDict {
	return nil
}

func (self *SDisk) GetStatus() string {
	// In_use Available Attaching Detaching Creating ReIniting All
	switch self.Properties.ProvisioningState {
	case "Updating":
		return models.DISK_ALLOCATING
	default:
		return models.DISK_READY
	}
}

func (self *SDisk) GetId() string {
	return self.ID
}

func (self *SRegion) getDisk(resourceGroup string, diskName string) (*SDisk, error) {
	return self.GetDisk(resourceGroup, diskName)
}

func (self *SDisk) Refresh() error {
	if disk, err := self.storage.zone.region.GetDisk(self.ResourceGroup, self.Name); err != nil {
		return cloudprovider.ErrNotFound
	} else {
		return jsonutils.Update(self, disk)
	}
}

func (self *SDisk) Delete() error {
	return self.storage.zone.region.deleteDisk(self.ID)
}

func (self *SDisk) Resize(size int64) error {
	//return self.storage.zone.region.resizeDisk(self.DiskId, size)
	return nil
}

func (self *SDisk) GetName() string {
	if len(self.Name) > 0 {
		return self.Name
	}
	return self.ID
}

func (self *SDisk) GetGlobalId() string {
	resourceGroup, _, _ := PareResourceGroupWithName(self.ID)
	return fmt.Sprintf("resourceGroups/%s/providers/disk/%s", resourceGroup, self.Name)
}

func (self *SDisk) IsEmulated() bool {
	return false
}

func (self *SDisk) GetIStorge() cloudprovider.ICloudStorage {
	return self.storage
}

func (self *SDisk) GetFsFormat() string {
	return ""
}

func (self *SDisk) GetIsNonPersistent() bool {
	return false
}

func (self *SDisk) GetDriver() string {
	return "scsi"
}

func (self *SDisk) GetCacheMode() string {
	return "none"
}

func (self *SDisk) GetMountpoint() string {
	return ""
}

func (self *SDisk) GetDiskFormat() string {
	return "vhd"
}

func (self *SDisk) GetDiskSizeMB() int {
	return int(self.Properties.DiskSizeGB) * 1024
}

func (self *SDisk) GetIsAutoDelete() bool {
	return false
}

func (self *SDisk) GetTemplateId() string {
	return self.Properties.CreationData.ImageReference.ID
}

func (self *SDisk) GetDiskType() string {
	if len(self.Properties.OsType) > 0 {
		return models.DISK_TYPE_SYS
	}
	return models.DISK_TYPE_DATA
}

func (self *SDisk) CreateISnapshot(name, desc string) (cloudprovider.ICloudSnapshot, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (self *SDisk) GetISnapshot(snapshotId string) (cloudprovider.ICloudSnapshot, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (self *SDisk) GetISnapshots() ([]cloudprovider.ICloudSnapshot, error) {
	return nil, cloudprovider.ErrNotImplemented
}
