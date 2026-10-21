package service

import (
	"errors"
	"strings"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"gorm.io/gorm"
)

type PeerService struct {
}

// FindById 根据id查找
func (ps *PeerService) FindById(id string) *model.Peer {
	p := &model.Peer{}
	DB.Where("id = ?", id).First(p)
	return p
}
func (ps *PeerService) FindByUuid(uuid string) *model.Peer {
	p := &model.Peer{}
	DB.Where("uuid = ?", uuid).First(p)
	return p
}
func (ps *PeerService) InfoByRowId(id uint) *model.Peer {
	p := &model.Peer{}
	DB.Where("row_id = ?", id).First(p)
	return p
}

// FindByUserIdAndUuid 根据用户id和uuid查找peer
func (ps *PeerService) FindByUserIdAndUuid(uuid string, userId uint) *model.Peer {
	p := &model.Peer{}
	DB.Where("uuid = ? and user_id = ?", uuid, userId).First(p)
	return p
}

var ErrPeerIdentityConflict = errors.New("peer identity conflict")

func validPeerIdentity(id, uuid string) bool {
	return strings.TrimSpace(id) != "" && strings.TrimSpace(uuid) != ""
}

func (ps *PeerService) FindByIdentity(id, uuid string) *model.Peer {
	peer := &model.Peer{}
	if validPeerIdentity(id, uuid) {
		DB.Where("id = ? AND uuid = ?", id, uuid).First(peer)
		if peer.Id != id || peer.Uuid != uuid {
			return &model.Peer{}
		}
	}
	return peer
}

func (ps *PeerService) FindByUserIdAndId(userId uint, id string) *model.Peer {
	peer := &model.Peer{}
	if userId != 0 {
		DB.Where("user_id = ? AND id = ?", userId, id).First(peer)
		if peer.Id != id {
			return &model.Peer{}
		}
	}
	return peer
}

// BindLoginIdentity may fill an administrator's ID-only placeholder, but never
// transfers another account's device or replaces an established ID/UUID pair.
func (ps *PeerService) BindLoginIdentity(deviceId, uuid string, userId uint) error {
	if !validPeerIdentity(deviceId, uuid) || userId == 0 {
		return ErrPeerIdentityConflict
	}
	Lock.Lock("peer-identity")
	defer Lock.UnLock("peer-identity")
	return DB.Transaction(func(tx *gorm.DB) error {
		var peers []model.Peer
		if err := tx.Where("id = ? OR uuid = ?", deviceId, uuid).Limit(2).Find(&peers).Error; err != nil {
			return err
		}
		if len(peers) == 0 {
			return tx.Create(&model.Peer{Id: deviceId, Uuid: uuid, UserId: userId}).Error
		}
		peer := &peers[0]
		if len(peers) != 1 || peer.Id != deviceId || (peer.Uuid != "" && peer.Uuid != uuid) || (peer.UserId != 0 && peer.UserId != userId) {
			return ErrPeerIdentityConflict
		}
		return tx.Model(peer).Updates(map[string]interface{}{"uuid": uuid, "user_id": userId}).Error
	})
}

// StoreSysinfo keeps anonymous stock clients compatible through first-use
// registration. Later reports must match that identity and cannot change ownership.
func (ps *PeerService) StoreSysinfo(report *model.Peer) error {
	if !validPeerIdentity(report.Id, report.Uuid) {
		return ErrPeerIdentityConflict
	}
	Lock.Lock("peer-identity")
	defer Lock.UnLock("peer-identity")
	return DB.Transaction(func(tx *gorm.DB) error {
		var peers []model.Peer
		if err := tx.Where("id = ? OR uuid = ?", report.Id, report.Uuid).Limit(2).Find(&peers).Error; err != nil {
			return err
		}
		if len(peers) == 0 {
			report.RowId = 0
			report.UserId = 0
			return tx.Create(report).Error
		}
		peer := &peers[0]
		if len(peers) != 1 || peer.Id != report.Id || peer.Uuid != report.Uuid {
			return ErrPeerIdentityConflict
		}
		return tx.Model(peer).Updates(map[string]interface{}{
			"cpu": report.Cpu, "hostname": report.Hostname, "memory": report.Memory,
			"os": report.Os, "username": report.Username, "version": report.Version,
		}).Error
	})
}

// UuidBindUserId is kept for existing callers; conflicting identities remain unchanged.
func (ps *PeerService) UuidBindUserId(deviceId string, uuid string, userId uint) error {
	return ps.BindLoginIdentity(deviceId, uuid, userId)
}

// UuidUnbindUserId 解绑用户id, 用于用户注销
func (ps *PeerService) UuidUnbindUserId(uuid string, userId uint) {
	peer := ps.FindByUserIdAndUuid(uuid, userId)
	if peer.RowId > 0 {
		DB.Model(peer).Update("user_id", 0)
	}
}

// EraseUserId 清除用户id, 用于用户删除
func (ps *PeerService) EraseUserId(userId uint) error {
	return DB.Model(&model.Peer{}).Where("user_id = ?", userId).Update("user_id", 0).Error
}

// ListByUserIds 根据用户id取列表
func (ps *PeerService) ListByUserIds(userIds []uint, page, pageSize uint) (res *model.PeerList) {
	res = &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	tx.Where("user_id in (?)", userIds)
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Peers)
	return
}

func (ps *PeerService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.PeerList) {
	res = &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Peers)
	return
}

// ListFilterByUserId 根据用户id过滤Peer列表
func (ps *PeerService) ListFilterByUserId(page, pageSize uint, where func(tx *gorm.DB), userId uint) (res *model.PeerList) {
	userWhere := func(tx *gorm.DB) {
		tx.Where("user_id = ?", userId)
		// 如果还有额外的筛选条件，执行它
		if where != nil {
			where(tx)
		}
	}
	return ps.List(page, pageSize, userWhere)
}

// Create 创建
func (ps *PeerService) Create(u *model.Peer) error {
	res := DB.Create(u).Error
	return res
}

// Delete 删除, 同时也应该删除token
func (ps *PeerService) Delete(u *model.Peer) error {
	uuid := u.Uuid
	err := DB.Delete(u).Error
	if err != nil {
		return err
	}
	// 删除token
	return AllService.UserService.FlushTokenByUuid(uuid)
}

// GetUuidListByIDs 根据ids获取uuid列表
func (ps *PeerService) GetUuidListByIDs(ids []uint) ([]string, error) {
	var uuids []string
	err := DB.Model(&model.Peer{}).
		Where("row_id in (?)", ids).
		Pluck("uuid", &uuids).Error
	//过滤uuids中的空字符串
	var newUuids []string
	for _, uuid := range uuids {
		if uuid != "" {
			newUuids = append(newUuids, uuid)
		}
	}
	return newUuids, err
}

// BatchDelete 批量删除, 同时也应该删除token
func (ps *PeerService) BatchDelete(ids []uint) error {
	uuids, err := ps.GetUuidListByIDs(ids)
	err = DB.Where("row_id in (?)", ids).Delete(&model.Peer{}).Error
	if err != nil {
		return err
	}
	// 删除token
	return AllService.UserService.FlushTokenByUuids(uuids)
}

// Update 更新
func (ps *PeerService) Update(u *model.Peer) error {
	return DB.Model(u).Updates(u).Error
}

func (ps *PeerService) UpdatePresence(peer *model.Peer, timestamp int64, ip string) error {
	return DB.Model(&model.Peer{}).
		Where("row_id = ? AND id = ? AND uuid = ?", peer.RowId, peer.Id, peer.Uuid).
		Updates(map[string]interface{}{"last_online_time": timestamp, "last_online_ip": ip}).Error
}
