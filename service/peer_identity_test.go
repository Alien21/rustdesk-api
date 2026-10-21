package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func identityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "identity.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Peer{}, &model.User{}, &model.UserToken{}, &model.LoginLog{}, &model.AddressBook{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldService, oldConfig, oldJWT, oldLock, oldLogger := DB, AllService, Config, Jwt, Lock, Logger
	New(&config.Config{App: config.App{TokenExpire: time.Hour}}, db, logrus.New(), jwt.NewJwt("", time.Hour), lock.NewLocal())
	t.Cleanup(func() {
		DB, AllService, Config, Jwt, Lock, Logger = oldDB, oldService, oldConfig, oldJWT, oldLock, oldLogger
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	return db
}

func TestPeerReportsPreserveIdentityAndOwnership(t *testing.T) {
	db := identityTestDB(t)
	owner, attacker := &model.User{Username: "owner"}, &model.User{Username: "attacker"}
	for _, user := range []*model.User{owner, attacker} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	report := &model.Peer{Id: "123456789", Uuid: "original-uuid", Hostname: "original-host"}
	if err := AllService.PeerService.StoreSysinfo(report); err != nil {
		t.Fatal(err)
	}
	if peer := AllService.PeerService.FindById(report.Id); peer.UserId != 0 {
		t.Fatal("anonymous report acquired an owner")
	}
	token := AllService.UserService.Login(owner, &model.LoginLog{UserId: owner.Id, DeviceId: report.Id, Uuid: report.Uuid, Client: "client"})
	if token.Token == "" || len(Jwt.Key) != 0 {
		t.Fatal("opaque native login failed")
	}
	if user, _ := AllService.UserService.InfoByAccessToken(token.Token); user.Id != owner.Id {
		t.Fatal("opaque token was not accepted")
	}
	if token.DeviceId != report.Id || token.DeviceUuid != report.Uuid {
		t.Fatal("valid native token lost its device identity")
	}
	for _, fake := range []*model.Peer{
		{Id: report.Id, Uuid: "attacker-uuid", Hostname: "forged"},
		{Id: "different-id", Uuid: report.Uuid, Hostname: "forged"},
		{Id: report.Id, Uuid: ""},
		{Id: "", Uuid: report.Uuid},
	} {
		if err := AllService.PeerService.StoreSysinfo(fake); !errors.Is(err, ErrPeerIdentityConflict) {
			t.Fatalf("forged report accepted: %+v: %v", fake, err)
		}
		AllService.UserService.Login(attacker, &model.LoginLog{UserId: attacker.Id, DeviceId: fake.Id, Uuid: fake.Uuid, Client: "client"})
	}
	for _, clientType := range []string{"client", model.LoginLogClientWeb} {
		conflicting := AllService.UserService.Login(attacker, &model.LoginLog{UserId: attacker.Id, DeviceId: report.Id, Uuid: report.Uuid, Client: clientType})
		if conflicting.DeviceId != "" || conflicting.DeviceUuid != "" {
			t.Fatalf("conflicting %s token retained a device claim", clientType)
		}
		saved := &model.UserToken{}
		if err := db.First(saved, conflicting.Id).Error; err != nil {
			t.Fatal(err)
		}
		if saved.DeviceId != "" || saved.DeviceUuid != "" {
			t.Fatal("conflicting device claim persisted")
		}
	}
	peer := AllService.PeerService.FindById(report.Id)
	if peer.UserId != owner.Id || peer.Uuid != report.Uuid || peer.Hostname != "original-host" {
		t.Fatalf("identity or owner changed: %+v", peer)
	}
	if err := AllService.PeerService.StoreSysinfo(&model.Peer{Id: report.Id, Uuid: report.Uuid, Hostname: "updated-host"}); err != nil {
		t.Fatal(err)
	}
	if peer := AllService.PeerService.FindById(report.Id); peer.Hostname != "updated-host" || peer.UserId != owner.Id {
		t.Fatalf("legitimate anonymous update failed: %+v", peer)
	}
	if err := AllService.UserService.Logout(owner, token.Token); err != nil {
		t.Fatal(err)
	}
	if peer := AllService.PeerService.FindById(report.Id); peer.UserId != 0 || peer.Uuid != report.Uuid {
		t.Fatalf("logout changed identity or retained ownership: %+v", peer)
	}
	if err := AllService.PeerService.StoreSysinfo(&model.Peer{Id: report.Id, Uuid: report.Uuid, Hostname: "after-logout"}); err != nil {
		t.Fatal(err)
	}
	if peer := AllService.PeerService.FindById(report.Id); peer.UserId != 0 || peer.Hostname != "after-logout" {
		t.Fatalf("anonymous report restored historical ownership: %+v", peer)
	}

	placeholder := &model.Peer{Id: "placeholder", UserId: owner.Id}
	if err := db.Create(placeholder).Error; err != nil {
		t.Fatal(err)
	}
	if err := AllService.PeerService.StoreSysinfo(&model.Peer{Id: placeholder.Id, Uuid: "placeholder-uuid"}); !errors.Is(err, ErrPeerIdentityConflict) {
		t.Fatal("anonymous report claimed ID-only placeholder")
	}
	if err := AllService.PeerService.BindLoginIdentity(placeholder.Id, "placeholder-uuid", attacker.Id); !errors.Is(err, ErrPeerIdentityConflict) {
		t.Fatal("another account claimed placeholder")
	}
	if err := AllService.PeerService.BindLoginIdentity(placeholder.Id, "placeholder-uuid", owner.Id); err != nil {
		t.Fatal(err)
	}
	if err := AllService.PeerService.StoreSysinfo(&model.Peer{Id: placeholder.Id, Uuid: "placeholder-uuid", Hostname: "provisioned"}); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyAddressBookMetadataIsScopedToOwner(t *testing.T) {
	db := identityTestDB(t)
	victim := &model.Peer{Id: "victim", Uuid: "victim-uuid", UserId: 10, Os: "Windows", Username: "private-user", Hostname: "private-host"}
	own := &model.Peer{Id: "own", Uuid: "own-uuid", UserId: 20, Os: "Linux", Username: "own-user", Hostname: "own-host"}
	for _, peer := range []*model.Peer{victim, own} {
		if err := db.Create(peer).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := AllService.AddressBookService.UpdateAddressBook([]*model.AddressBook{{Id: victim.Id}, {Id: own.Id}}, 20); err != nil {
		t.Fatal(err)
	}
	leaked := AllService.AddressBookService.InfoByUserIdAndId(20, victim.Id)
	if leaked.Username != "" || leaked.Hostname != "" || leaked.Platform != "" {
		t.Fatalf("foreign metadata leaked: %+v", leaked)
	}
	enriched := AllService.AddressBookService.InfoByUserIdAndId(20, own.Id)
	if enriched.Username != own.Username || enriched.Hostname != own.Hostname || enriched.Platform != "Linux" {
		t.Fatalf("own metadata no longer enriched: %+v", enriched)
	}
}

func TestPeerLookupsRequireByteExactIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "case-insensitive.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	// SQLite NOCASE models a case-insensitive MySQL identifier collation.
	if err := db.Exec("CREATE TABLE peers (row_id integer primary key, id text COLLATE NOCASE, uuid text COLLATE NOCASE, user_id integer)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO peers (row_id,id,uuid,user_id) VALUES (1,'CaseSensitiveID','CaseSensitiveUUID',10)").Error; err != nil {
		t.Fatal(err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB; sqlDB, _ := db.DB(); sqlDB.Close() })
	peerService := &PeerService{}
	if peer := peerService.FindByIdentity("CaseSensitiveID", "CaseSensitiveUUID"); peer.RowId != 1 {
		t.Fatal("exact identity lookup failed")
	}
	for _, identity := range [][2]string{
		{"casesensitiveid", "CaseSensitiveUUID"},
		{"CaseSensitiveID", "casesensitiveuuid"},
	} {
		if peer := peerService.FindByIdentity(identity[0], identity[1]); peer.RowId != 0 {
			t.Fatalf("case-insensitive identity accepted: %+v", peer)
		}
	}
	if peer := peerService.FindByUserIdAndId(10, "casesensitiveid"); peer.RowId != 0 {
		t.Fatal("case-insensitive metadata ID accepted")
	}
	if peer := peerService.FindByUserIdAndId(10, "CaseSensitiveID"); peer.RowId != 1 {
		t.Fatal("exact owner metadata lookup failed")
	}
}
