package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func peerAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "peer-api.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Peer{}, &model.User{}, &model.UserToken{}, &model.LoginLog{}, &model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldService, oldConfig, oldJWT, oldLock, oldLogger := service.DB, service.AllService, service.Config, service.Jwt, service.Lock, service.Logger
	service.New(&config.Config{App: config.App{TokenExpire: time.Hour}}, db, logrus.New(), jwt.NewJwt("", time.Hour), lock.NewLocal())
	t.Cleanup(func() {
		service.DB, service.AllService, service.Config, service.Jwt, service.Lock, service.Logger = oldDB, oldService, oldConfig, oldJWT, oldLock, oldLogger
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	return db
}

func postPeerJSON(t *testing.T, router http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.10:12345"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestAnonymousReportsRejectChangedIdentity(t *testing.T) {
	peerAPITestDB(t)
	router := gin.New()
	router.POST("/api/sysinfo", (&Peer{}).SysInfo)
	router.POST("/api/heartbeat", (&Index{}).Heartbeat)
	valid := `{"id":"123456789","uuid":"original-uuid","hostname":"original-host","version":"1.5.0"}`
	if response := postPeerJSON(t, router, "/api/sysinfo", valid); response.Code != http.StatusOK || response.Body.String() != "SYSINFO_UPDATED" {
		t.Fatalf("stock sysinfo failed: %d %s", response.Code, response.Body.String())
	}
	if err := service.AllService.PeerService.BindLoginIdentity("123456789", "original-uuid", 10); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		`{"id":"123456789","uuid":"attacker-uuid","hostname":"forged"}`,
		`{"id":"different-id","uuid":"original-uuid","hostname":"forged"}`,
		`{"id":"123456789","uuid":""}`,
	} {
		if response := postPeerJSON(t, router, "/api/sysinfo", invalid); response.Code != http.StatusOK || response.Body.String() != "ID_NOT_FOUND" {
			t.Fatalf("sysinfo rejection changed wire format: %d %s", response.Code, response.Body.String())
		}
		if response := postPeerJSON(t, router, "/api/heartbeat", invalid); response.Code != http.StatusOK || response.Body.String() != "{}" {
			t.Fatalf("heartbeat rejection changed wire format: %d %s", response.Code, response.Body.String())
		}
	}
	peer := service.AllService.PeerService.FindById("123456789")
	if peer.Uuid != "original-uuid" || peer.UserId != 10 || peer.Hostname != "original-host" || peer.LastOnlineTime != 0 || peer.LastOnlineIp != "" {
		t.Fatalf("forged report mutated peer: %+v", peer)
	}
	if response := postPeerJSON(t, router, "/api/heartbeat", `{"id":"123456789","uuid":"original-uuid","ver":1005001}`); response.Body.String() != "{}" {
		t.Fatal(response.Body.String())
	}
	peer = service.AllService.PeerService.FindById("123456789")
	if peer.LastOnlineTime == 0 || peer.LastOnlineIp != "192.0.2.10" {
		t.Fatalf("valid 1.5.1 heartbeat failed: %+v", peer)
	}
}

func TestPeerAddMetadataIsScopedToAuthenticatedActor(t *testing.T) {
	db := peerAPITestDB(t)
	owner, actor := &model.User{Username: "owner", GroupId: 1}, &model.User{Username: "writer", GroupId: 1}
	for _, user := range []*model.User{owner, actor} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	victim := &model.Peer{Id: "victim", Uuid: "victim-uuid", UserId: owner.Id, Username: "private-user", Hostname: "private-host", Os: "Windows"}
	own := &model.Peer{Id: "own", Uuid: "own-uuid", UserId: actor.Id, Username: "own-user", Hostname: "own-host", Os: "Linux"}
	for _, peer := range []*model.Peer{victim, own} {
		if err := db.Create(peer).Error; err != nil {
			t.Fatal(err)
		}
	}
	collection := &model.AddressBookCollection{UserId: owner.Id, Name: "Shared"}
	if err := db.Create(collection).Error; err != nil {
		t.Fatal(err)
	}
	rule := &model.AddressBookCollectionRule{UserId: owner.Id, CollectionId: collection.Id, Type: model.ShareAddressBookRuleTypePersonal, ToId: actor.Id, Rule: model.ShareAddressBookRuleRuleReadWrite}
	if err := db.Create(rule).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("curUser", actor) })
	router.POST("/api/ab/peer/add/:guid", (&Ab{}).PeerAdd)
	cases := []struct {
		guid, id    string
		destination uint
		own         bool
	}{
		{fmt.Sprintf("1-%d-0", actor.Id), victim.Id, actor.Id, false},
		{fmt.Sprintf("1-%d-%d", owner.Id, collection.Id), victim.Id, owner.Id, false},
		{fmt.Sprintf("1-%d-0", actor.Id), own.Id, actor.Id, true},
	}
	for _, item := range cases {
		response := postPeerJSON(t, router, "/api/ab/peer/add/"+item.guid, fmt.Sprintf(`{"id":%q}`, item.id))
		if response.Code != http.StatusOK || response.Body.String() != "" {
			t.Fatalf("PeerAdd failed: %d %s", response.Code, response.Body.String())
		}
		address := service.AllService.AddressBookService.InfoByUserIdAndId(item.destination, item.id)
		if address.RowId == 0 {
			t.Fatal("address was not added")
		}
		if !item.own && (address.Username != "" || address.Hostname != "" || address.Platform != "") {
			t.Fatalf("foreign metadata exposed to writer: %+v", address)
		}
		if item.own && (address.Username != own.Username || address.Hostname != own.Hostname || address.Platform != "Linux") {
			t.Fatalf("own metadata enrichment failed: %+v", address)
		}
	}
}
