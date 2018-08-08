package hostdrivers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yunionio/jsonutils"
	"github.com/yunionio/log"
	"github.com/yunionio/onecloud/pkg/cloudcommon/db/taskman"
	"github.com/yunionio/onecloud/pkg/compute/models"
	"github.com/yunionio/onecloud/pkg/util/httputils"
)

type SKVMHostDriver struct {
}

func init() {
	driver := SKVMHostDriver{}
	models.RegisterHostDriver(&driver)
}

func (self *SKVMHostDriver) GetHostType() string {
	return models.HOST_TYPE_HYPERVISOR
}

func (self *SKVMHostDriver) CheckAndSetCacheImage(ctx context.Context, host *models.SHost, storageCache *models.SStoragecache, scimg *models.SStoragecachedimage, task taskman.ITask) error {
	params := task.GetParams()
	imageId, err := params.GetString("image_id")
	if err != nil {
		return err
	}
	isForce := jsonutils.QueryBoolean(params, "is_force", false)
	obj, err := models.CachedimageManager.FetchById(imageId)
	if err != nil {
		return err
	}
	cacheImage := obj.(*models.SCachedimage)
	srcHostCacheImage, err := cacheImage.ChooseSourceStoragecacheInRange(models.HOST_TYPE_HYPERVISOR, []string{host.Id}, []*models.SZone{host.GetZone()})
	if err != nil {
		return err
	}

	content := jsonutils.NewDict()
	content.Add(jsonutils.NewString(imageId), "image_id")
	if srcHostCacheImage != nil {
		err = srcHostCacheImage.AddDownloadRefcount()
		if err != nil {
			return err
		}
		srcHost, err := srcHostCacheImage.GetHost()
		if err != nil {
			return err
		}
		srcUrl := fmt.Sprintf("%s/download/images/%s", srcHost.ManagerUri, imageId)
		content.Add(jsonutils.NewString(srcUrl), "src_url")
	}
	url := fmt.Sprintf("%s/disks/image_cache", host.ManagerUri)

	if isForce {
		content.Add(jsonutils.NewBool(true), "is_force")
	}
	content.Add(jsonutils.NewString(storageCache.Id), "storagecache_id")
	body := jsonutils.NewDict()
	body.Add(content, "disk")
	header := http.Header{}
	header.Set("X-Auth-Token", task.GetUserCred().GetTokenString())
	header.Set("X-Task-Id", task.GetTaskId())
	header.Set("X-Region-Version", "v2")
	_, _, err = httputils.JSONRequest(httputils.GetDefaultClient(), ctx, "POST", url, header, body, false)
	if err != nil {
		return err
	}
	return nil
}

func (self *SKVMHostDriver) RequestAllocateDiskOnStorage(host *models.SHost, storage *models.SStorage, disk *models.SDisk, task taskman.ITask, content *jsonutils.JSONDict) error {
	header := http.Header{}
	header.Add("X-Task-Id", task.GetTaskId())
	header.Add("X-Region-Version", "v2")
	url := fmt.Sprintf("/disks/%s/create/%s", storage.Id, disk.Id)
	body := jsonutils.NewDict()
	body.Add(content, "disk")
	_, err := host.Request(task.GetUserCred(), "POST", url, header, body)
	return err
}

func (self *SKVMHostDriver) RequestDeallocateDiskOnHost(host *models.SHost, storage *models.SStorage, disk *models.SDisk, task taskman.ITask) error {
	log.Infof("Deallocating disk on host %s", host.GetName())
	header := http.Header{}
	header.Add("X-Task-Id", task.GetTaskId())
	header.Add("X-Region-Version", "v2")
	url := fmt.Sprintf("/disks/%s/delete/%s", storage.Id, disk.Id)
	body := jsonutils.NewDict()
	_, err := host.Request(task.GetUserCred(), "POST", url, header, body)
	return err
}

func (self *SKVMHostDriver) RequestResizeDiskOnHost(host *models.SHost, storage *models.SStorage, disk *models.SDisk, size int64, task taskman.ITask) error {
	header := http.Header{}
	header.Add("X-Task-Id", task.GetTaskId())
	header.Add("X-Region-Version", "v2")
	url := fmt.Sprintf("/disks/%s/resize/%s", storage.Id, disk.Id)
	body := jsonutils.NewDict()
	content := jsonutils.NewDict()
	content.Add(jsonutils.NewInt(size), "size")
	body.Add(content, "disk")
	_, err := host.Request(task.GetUserCred(), "POST", url, header, body)
	return err
}

func (self *SKVMHostDriver) RequestResizeDiskOnHostOnline(host *models.SHost, storage *models.SStorage, disk *models.SDisk, size int64, task taskman.ITask) error {
	self.RequestResizeDiskOnHost(host, storage, disk, size, task)
	header := http.Header{}
	header.Add("X-Task-Id", task.GetTaskId())
	header.Add("X-Region-Version", "v2")
	for _, guest := range disk.GetAttachedGuests() {
		guestdisk := guest.GetGuestDisk(disk.GetId())
		url := fmt.Sprintf("/servers/%s/monitor", guest.GetId())
		body := jsonutils.NewDict()
		cmd := fmt.Sprintf("block_resize drive_%d %dM", guestdisk.Index, size)
		body.Add(jsonutils.NewString(cmd), "cmd")
		host.Request(task.GetUserCred(), "POST", url, header, body)
	}
	return nil
}
