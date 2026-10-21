package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminCommandsAndLogoutRequireAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&model.User{}, &model.UserToken{}, &model.Peer{}, &model.ServerCmd{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldService, oldConfig, oldJWT, oldLock, oldLogger := service.DB, service.AllService, service.Config, service.Jwt, service.Lock, service.Logger
	oldGlobalLogger, oldLocalizer := global.Logger, global.Localizer
	service.New(&config.Config{}, db, logrus.New(), jwt.NewJwt("", time.Hour), lock.NewLocal())
	global.Logger = logrus.New()
	bundle := i18n.NewBundle(language.English)
	global.Localizer = func(string) *i18n.Localizer { return i18n.NewLocalizer(bundle, "en") }
	t.Cleanup(func() {
		service.DB, service.AllService, service.Config, service.Jwt, service.Lock, service.Logger = oldDB, oldService, oldConfig, oldJWT, oldLock, oldLogger
		global.Logger, global.Localizer = oldGlobalLogger, oldLocalizer
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	regular, adminFlag := false, true
	users := []*model.User{{Username: "regular", IsAdmin: &regular}, {Username: "admin", IsAdmin: &adminFlag}}
	for _, u := range users {
		if err = db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
		if err = db.Create(&model.UserToken{UserId: u.Id, Token: u.Username, ExpiredAt: time.Now().Add(7 * 24 * time.Hour).Unix()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	Init(r)
	request := func(method, path, token string) int {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("api-token", token)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: HTTP %d %s", path, w.Code, w.Body.String())
		}
		var body struct{ Code int }
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Code
	}
	for _, endpoint := range []struct{ method, path string }{
		{"POST", "sendCmd"}, {"GET", "cmdList"}, {"POST", "cmdCreate"}, {"POST", "cmdUpdate"}, {"POST", "cmdDelete"},
	} {
		for _, token := range []string{"", "invalid", "regular"} {
			if code := request(endpoint.method, "/api/admin/rustdesk/"+endpoint.path, token); code != 403 {
				t.Fatalf("%s accepted token %q: %d", endpoint.path, token, code)
			}
		}
	}
	if code := request("GET", "/api/admin/rustdesk/cmdList", "admin"); code != 0 {
		t.Fatalf("admin command list denied: %d", code)
	}
	if code := request("POST", "/api/admin/logout", ""); code != 403 {
		t.Fatalf("anonymous logout accepted: %d", code)
	}
	if code := request("POST", "/api/admin/logout", "regular"); code != 0 {
		t.Fatalf("authenticated logout failed: %d", code)
	}
	if user, _ := service.AllService.UserService.InfoByAccessToken("regular"); user.Id != 0 {
		t.Fatal("logout did not revoke opaque token")
	}
}
