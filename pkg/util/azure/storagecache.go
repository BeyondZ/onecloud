package azure

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/compute/options"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"

	storageaccount "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2018-02-01/storage"
	"github.com/Azure/azure-sdk-for-go/storage"
)

const (
	DefaultStorageAccount string = "storage"
	DefaultBlobContainer  string = "image-cache"

	DefaultReadBlockSize uint64 = 4 * 1024 * 1024
)

type SStoragecache struct {
	region *SRegion

	iimages []cloudprovider.ICloudImage
}

func (self *SStoragecache) GetMetadata() *jsonutils.JSONDict {
	return nil
}

func (self *SStoragecache) GetId() string {
	return fmt.Sprintf("%s-%s", self.region.client.providerId, self.region.GetId())
}

func (self *SStoragecache) GetName() string {
	return fmt.Sprintf("%s-%s", self.region.client.providerName, self.region.GetId())
}

func (self *SStoragecache) GetStatus() string {
	return "available"
}

func (self *SStoragecache) Refresh() error {
	return nil
}

func (self *SStoragecache) GetGlobalId() string {
	return fmt.Sprintf("%s-%s", self.region.client.providerId, self.region.GetGlobalId())
}

func (self *SStoragecache) IsEmulated() bool {
	return false
}

func (self *SStoragecache) GetManagerId() string {
	return self.region.client.providerId
}

func (self *SStoragecache) fetchImages() error {
	if images, err := self.region.GetImages(); err != nil {
		return err
	} else {
		self.iimages = make([]cloudprovider.ICloudImage, len(images))
		for i := 0; i < len(images); i++ {
			images[i].storageCache = self
			self.iimages[i] = &images[i]
		}
	}
	return nil
}

func (self *SStoragecache) GetIImages() ([]cloudprovider.ICloudImage, error) {
	if self.iimages == nil {
		if err := self.fetchImages(); err != nil {
			return nil, err
		}
	}
	return self.iimages, nil
}

func (self *SStoragecache) UploadImage(userCred mcclient.TokenCredential, imageId string, extId string, isForce bool) (string, error) {
	if len(extId) > 0 {
		status, _ := self.region.GetImageStatus(extId)
		if status == ImageStatusAvailable && !isForce {
			return extId, nil
		}
	}
	return self.uploadImage(userCred, imageId, isForce)
}

func (self *SRegion) CreateStorageAccount(resourceGroup, storageAccount string) error {
	storageClinet := storageaccount.NewAccountsClientWithBaseURI(self.client.baseUrl, self.SubscriptionID)
	storageClinet.Authorizer = self.client.authorizer
	sku := storageaccount.Sku{Name: storageaccount.SkuName("Standard_GRS")}
	params := storageaccount.AccountCreateParameters{Sku: &sku, Location: &self.Name, Kind: storageaccount.Kind("Storage")}
	if len(resourceGroup) == 0 {
		resourceGroup = DefaultResourceGroup["storage"]
	}
	if len(storageAccount) == 0 {
		storageAccount = fmt.Sprintf("%s%s", self.Name, DefaultStorageAccount)
	}
	if result, err := storageClinet.Create(context.Background(), resourceGroup, storageAccount, params); err != nil {
		return err
	} else if err := result.WaitForCompletion(context.Background(), storageClinet.Client); err != nil {
		return err
	}
	return nil
}

func (self *SRegion) checkStorageContainer(storageAccount, accessKey, containerName string) error {
	if client, err := storage.NewBasicClientOnSovereignCloud(storageAccount, accessKey, self.client.env); err != nil {
		return err
	} else {
		blob := client.GetBlobService()
		container := blob.GetContainerReference(containerName)
		option := storage.CreateContainerOptions{Timeout: 10, Access: storage.ContainerAccessType("")}
		_, err := container.CreateIfNotExists(&option)
		return err
	}
	return nil
}

func (self *SRegion) isStorageAccountExist(resourceGroup, storageAccount string) bool {
	storageClinet := storageaccount.NewAccountsClientWithBaseURI(self.client.baseUrl, self.SubscriptionID)
	storageClinet.Authorizer = self.client.authorizer
	if _, err := storageClinet.GetProperties(context.Background(), resourceGroup, storageAccount); err != nil {
		return false
	}
	return true
}

func (self *SRegion) getStorageAccountKey(resourceGroup, storageAccount string) (string, error) {
	storageClinet := storageaccount.NewAccountsClientWithBaseURI(self.client.baseUrl, self.SubscriptionID)
	storageClinet.Authorizer = self.client.authorizer
	if result, err := storageClinet.ListKeys(context.Background(), resourceGroup, storageAccount); err != nil {
		return "", err
	} else {
		for _, key := range *result.Keys {
			permission := strings.ToLower(string(key.Permissions))
			if permission == "full" {
				return *key.Value, nil
			}
		}
	}
	return "", fmt.Errorf("not find storage account accessKey")
}

func (self *SRegion) CheckBlobContainer(resourceGroup, storageAccount, blobName string) error {
	if len(resourceGroup) == 0 {
		resourceGroup = DefaultResourceGroup["storage"]
	}
	if len(storageAccount) == 0 {
		storageAccount = fmt.Sprintf("%s%s", self.Name, DefaultStorageAccount)
	}
	if len(blobName) == 0 {
		blobName = DefaultBlobContainer
	}
	if err := self.client.fetchAzueResourceGroup(); err != nil {
		return err
	} else if !self.isStorageAccountExist(resourceGroup, storageAccount) {
		if err := self.CreateStorageAccount(resourceGroup, storageAccount); err != nil {
			return err
		}
	}
	if accessKey, err := self.getStorageAccountKey(resourceGroup, storageAccount); err != nil {
		return err
	} else {
		return self.checkStorageContainer(storageAccount, accessKey, blobName)
	}
}

type BlobProperties struct {
	LastModified          time.Time
	ContentMD5            string
	ContentLength         int64
	ContentType           string
	ContentEncoding       string
	CacheControl          string
	ContentLanguage       string
	ContentDisposition    string
	BlobType              string
	SequenceNumber        int64
	CopyID                string
	CopyStatus            string
	CopySource            string
	CopyProgress          string
	CopyCompletionTime    time.Time
	CopyStatusDescription string
	LeaseStatus           string
	LeaseState            string
	LeaseDuration         string
	ServerEncrypted       bool
	IncrementalCopy       bool
}

type BlobMetadata map[string]string

type ContainerProperties struct {
	LastModified  string
	Etag          string
	LeaseStatus   string
	LeaseState    string
	LeaseDuration string
	PublicAccess  string
}

type Container struct {
	Name       string
	Metadata   map[string]string
	Properties ContainerProperties
}

type Blob struct {
	Container  Container
	Name       string
	Snapshot   time.Time
	Properties BlobProperties
	Metadata   BlobMetadata
}

func (self *SRegion) getContainerFiles(storageAccount, accessKey, containerName string) ([]Blob, error) {
	if client, err := storage.NewBasicClientOnSovereignCloud(storageAccount, accessKey, self.client.env); err != nil {
		return nil, err
	} else {
		blob := client.GetBlobService()
		container := blob.GetContainerReference(containerName)
		params := storage.ListBlobsParameters{}
		blobs := make([]Blob, 0)
		if result, err := container.ListBlobs(params); err != nil {
			return nil, err
		} else if err := jsonutils.Update(&blobs, result.Blobs); err != nil {
			return nil, err
		} else {
			return blobs, nil
		}
	}
}

func (self *SRegion) ListContainerFiles(resourceGroup, storageAccount, blobName string) ([]Blob, error) {
	if len(resourceGroup) == 0 {
		resourceGroup = DefaultResourceGroup["storage"]
	}
	if len(storageAccount) == 0 {
		storageAccount = fmt.Sprintf("%s%s", self.Name, DefaultStorageAccount)
	}
	if len(blobName) == 0 {
		blobName = DefaultBlobContainer
	}
	if accessKey, err := self.getStorageAccountKey(resourceGroup, storageAccount); err != nil {
		return nil, err
	} else {
		return self.getContainerFiles(storageAccount, accessKey, blobName)
	}
}

func (self *SRegion) uploadContainerFileByReader(storageAccount, accessKey, containerName, fileName string, content io.Reader, size int64) (string, error) {
	if client, err := storage.NewBasicClientOnSovereignCloud(storageAccount, accessKey, self.client.env); err != nil {
		return "", err
	} else {
		blob := client.GetBlobService()
		container := blob.GetContainerReference(containerName)
		blobClient := container.GetBlobReference(fileName)
		if _, err := blobClient.DeleteIfExists(&storage.DeleteBlobOptions{}); err != nil {
			return "", err
		}
		blobClient.Properties.ContentLength = size
		if err := blobClient.PutPageBlob(&storage.PutBlobOptions{}); err != nil {
			return "", err
		}
		var readed uint64 = 0
		for i := 0; i < int(uint64(size)/DefaultReadBlockSize); i++ {
			if err := blobClient.WriteRange(storage.BlobRange{Start: readed, End: readed + DefaultReadBlockSize - 1}, content, &storage.PutPageOptions{}); err != nil {
				return "", err
			}
			readed += DefaultReadBlockSize
			log.Debugf("Upload %s %f%% to %s", fileName, float64(readed)/float64(size)*100, storageAccount)
		}
		if extraSize := uint64(size) % DefaultReadBlockSize; extraSize > 0 {
			log.Debugf("Upload %s extra size: %d to %s", fileName, extraSize, storageAccount)
			if err := blobClient.WriteRange(storage.BlobRange{Start: readed, End: readed + extraSize - 1}, content, &storage.PutPageOptions{}); err != nil {
				return "", err
			}
		}
		log.Debugf("Upload %s complate", fileName)
		return blobClient.GetURL(), nil
	}
}

func (self *SRegion) uploadContainerFileByPath(storageAccount, accessKey, containerName, filePath string) (string, error) {
	if f, err := os.Open(filePath); err != nil {
		return "", err
	} else {
		defer f.Close()
		if finfo, err := f.Stat(); err != nil {
			return "", err
		} else {
			return self.uploadContainerFileByReader(storageAccount, accessKey, containerName, finfo.Name(), f, finfo.Size())
		}
	}
}

func (self *SRegion) UploadContainerFiles(resourceGroup, storageAccount, containerName, filePath string) (string, error) {
	if len(resourceGroup) == 0 {
		resourceGroup = DefaultResourceGroup["storage"]
	}
	if len(storageAccount) == 0 {
		storageAccount = fmt.Sprintf("%s%s", self.Name, DefaultStorageAccount)
	}
	if len(containerName) == 0 {
		containerName = DefaultBlobContainer
	}
	if accessKey, err := self.getStorageAccountKey(resourceGroup, storageAccount); err != nil {
		return "", err
	} else {
		return self.uploadContainerFileByPath(storageAccount, accessKey, containerName, filePath)
	}
}

func (self *SStoragecache) uploadImage(userCred mcclient.TokenCredential, imageId string, isForce bool) (string, error) {
	s := auth.GetAdminSession(options.Options.Region, "")

	if meta, reader, err := modules.Images.Download(s, imageId); err != nil {
		return "", err
	} else {
		// {"checksum":"d0ab0450979977c6ada8d85066a6e484","container_format":"bare","created_at":"2018-08-10T04:18:07","deleted":"False","disk_format":"vhd","id":"64189033-3ad4-413c-b074-6bf0b6be8508","is_public":"False","min_disk":"0","min_ram":"0","name":"centos-7.3.1611-20180104.vhd","owner":"5124d80475434da8b41fee48d5be94df","properties":{"os_arch":"x86_64","os_distribution":"CentOS","os_type":"Linux","os_version":"7.3.1611-VHD"},"protected":"False","size":"2028505088","status":"active","updated_at":"2018-08-10T04:20:59"}
		log.Infof("meta data %s", meta)

		osType, _ := meta.GetString("properties", "os_type")
		imageNameOnBlob, _ := meta.GetString("name")
		if !strings.HasSuffix(imageNameOnBlob, ".vhd") {
			imageNameOnBlob = fmt.Sprintf("%s.vhd", imageNameOnBlob)
		}

		storageAccount := fmt.Sprintf("%s%s", self.region.Name, DefaultStorageAccount)

		if err := self.region.CheckBlobContainer(DefaultResourceGroup["storage"], storageAccount, DefaultBlobContainer); err != nil {
			return "", err
		}

		size, _ := meta.Int("size")
		accessKey, err := self.region.getStorageAccountKey(DefaultResourceGroup["storage"], storageAccount)
		if err != nil {
			return "", err
		}

		blobURI, err := self.region.uploadContainerFileByReader(storageAccount, accessKey, DefaultBlobContainer, imageNameOnBlob, reader, size)
		if err != nil {
			log.Errorf("uploadContainerFileByReader error: %v", err)
			return "", err
		}

		imageBaseName := imageId
		if imageBaseName[0] >= '0' && imageBaseName[0] <= '9' {
			imageBaseName = fmt.Sprintf("img%s", imageId)
		}
		imageName := imageBaseName
		nameIdx := 1

		// check image name, avoid name conflict
		for {
			if _, err = self.region.GetImageByName(imageName); err != nil {
				if err == cloudprovider.ErrNotFound {
					break
				} else {
					return "", err
				}
			}
			imageName = fmt.Sprintf("%s-%d", imageBaseName, nameIdx)
			nameIdx += 1
		}

		if image, err := self.region.CreateImageByBlob(imageName, osType, blobURI); err != nil {
			return "", err
		} else {
			return image.GetId(), nil
		}
	}
}

func (self *SStoragecache) CreateIImage(snapshoutId, imageName, imageDesc string) (cloudprovider.ICloudImage, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (self *SStoragecache) DownloadImage(userCred mcclient.TokenCredential, imageId string, extId string) (jsonutils.JSONObject, error) {
	return nil, cloudprovider.ErrNotImplemented
}
