package logclient

import (
	"fmt"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
)

const (
	ACT_VM_RESET_PSWD                = "重置密码"
	ACT_VM_REBUILD                   = "重装系统"
	ACT_VM_START                     = "开机"
	ACT_VM_STOP                      = "关机"
	ACT_VM_PURGE                     = "清除"
	ACT_VM_CHANGE_FLAVOR             = "调整配置"
	ACT_VM_SYNC_CONF                 = "同步配置"
	ACT_GUEST_ATTACH_ISOLATED_DEVICE = "挂载透传设备"
	ACT_GUEST_DETACH_ISOLATED_DEVICE = "卸载透传设备"
	ACT_VM_SYNC_STATUS               = "同步状态"
	ACT_CREATE                       = "创建"
	ACT_DELETE                       = "删除"
	ACT_UPDATE                       = "更新"
	ACT_RESERVE_IP                   = "预留IP"
	ACT_RELEASE_IP                   = "释放IP"
	ACT_CANCEL_DELETE                = "恢复"
	ACT_UNCACHED_IMAGE               = "清除缓存"
	ACT_ENABLE                       = "启用"
	ACT_DISABLE                      = "禁用"
	ACT_ONLINE                       = "上线"
	ACT_OFFLINE                      = "下线"
	ACT_PUBLIC                       = "设为共享"
	ACT_PRIVATE                      = "设为私有"
	ACT_MERGE                        = "合并"
	ACT_SPLIT                        = "分割"
	ACT_ALLOCATE                     = "分配"
	ACT_BM_MAINTENANCE               = "进入离线状态"
	ACT_BM_UNMAINTENANCE             = "退出离线状态"
	ACT_BM_CONVERT_HYPER             = "转换为宿主机"
	ACT_BM_UNCONVERT_HYPER           = "转换为受管物理机"
	ACT_ADDTAG                       = "添加标签"
	ACT_RMTAG                        = "删除标签"
	ACT_RESIZE                       = "扩容"
	ACT_VM_ATTACH_DISK               = "挂载磁盘"
	ACT_VM_DETACH_DISK               = "卸载磁盘"
)

type IObject interface {
	GetId() string
	GetName() string
	Keyword() string
}

func AddActionLog(userCred mcclient.TokenCredential, action, notes string, obj IObject, e string) {

	token := userCred
	logentry := jsonutils.NewDict()
	logentry.Add(jsonutils.NewString(obj.GetName()), "obj_name")
	logentry.Add(jsonutils.NewString(obj.Keyword()), "obj_type")
	logentry.Add(jsonutils.NewString(obj.GetId()), "obj_id")
	logentry.Add(jsonutils.NewString(action), "action")
	logentry.Add(jsonutils.NewString(token.GetUserId()), "user_id")
	logentry.Add(jsonutils.NewString(token.GetUserName()), "user")
	logentry.Add(jsonutils.NewString(token.GetTenantId()), "tenant_id")
	logentry.Add(jsonutils.NewString(token.GetTenantName()), "tenant")

	// TODO delete following line later
	notes = "[a2]" + notes

	s := auth.GetSession(userCred, "", "")

	if len(e) > 0 {
		// 失败日志
		logentry.Add(jsonutils.JSONFalse, "success")
		notes = fmt.Sprintf("%s%s", notes, e)
		logentry.Add(jsonutils.NewString(notes), "notes")
	} else {
		// 成功日志
		logentry.Add(jsonutils.JSONTrue, "success")
		logentry.Add(jsonutils.NewString(notes), "notes")
	}

	_, err := modules.Actions.Create(s, logentry)
	if err != nil {
		fmt.Printf("create action log failed %s", err)
	} else {
		fmt.Println("create action log success")
	}
}
