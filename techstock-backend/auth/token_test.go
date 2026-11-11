package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("กุญแจลับสำหรับทดสอบเท่านั้น-ยาวพอแน่นอน")

func sign(t *testing.T, m jwt.SigningMethod, key any, c jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(m, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestForgedTickets(t *testing.T) {
	iss := &Issuer{Secret: secret, TTL: time.Hour}
	genuine, _ := iss.Issue(1, "member")
	now := time.Now()
	good := jwt.MapClaims{"sub": "1", "role": "admin", "exp": now.Add(time.Hour).Unix()}

	// แก้ member เป็น admin ในเนื้อความ แต่คงลายเซ็นเดิม
	p := strings.Split(genuine, ".")
	body, _ := base64.RawURLEncoding.DecodeString(p[1])
	body = []byte(strings.Replace(string(body), "member", "admin", 1))
	edited := p[0] + "." + base64.RawURLEncoding.EncodeToString(body) + "." + p[2]

	if _, role, err := iss.Parse(genuine); err != nil || role != "member" {
		t.Fatalf("ตั๋วจริงต้องผ่าน แต่ได้ %v", err)
	}

	cases := map[string]string{
		"แก้เนื้อความ":            edited,
		"เซ็นด้วยกุญแจอื่น":       sign(t, jwt.SigningMethodHS256, []byte("กุญแจของคนอื่น-ยาวพอเหมือนกันนะ"), good),
		"อ้างว่าไม่ต้องเซ็น":      sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, good),
		"กุญแจถูกแต่วิธีเซ็นอื่น": sign(t, jwt.SigningMethodHS512, secret, good),
		"ไม่มีวันหมดอายุ":         sign(t, jwt.SigningMethodHS256, secret, jwt.MapClaims{"sub": "1", "role": "admin"}),
		"หมดอายุแล้ว":             sign(t, jwt.SigningMethodHS256, secret, jwt.MapClaims{"sub": "1", "role": "member", "exp": now.Add(-time.Minute).Unix()}),
	}
	for name, token := range cases {
		if _, _, err := iss.Parse(token); err == nil {
			t.Errorf("%s ผ่านเข้ามาได้ ทั้งที่ต้องถูกปฏิเสธ", name)
		}
	}
}
