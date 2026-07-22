package application

import "testing"

func TestCanonicalUploadFolderScopesTenantAndRejectsTraversal(t *testing.T) {
	tests := []struct {
		folder string
		want   string
		ok     bool
	}{
		{folder: "applications/app-1", want: "school-a/applications/app-1", ok: true},
		{folder: "school-a/logos", want: "school-a/logos", ok: true},
		{folder: "../school-b", ok: false},
		{folder: "school-a\\logos", ok: false},
	}
	for _, tc := range tests {
		got, err := canonicalUploadFolder("school-a", tc.folder)
		if tc.ok && (err != nil || got != tc.want) {
			t.Fatalf("folder %q: got=%q err=%v", tc.folder, got, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("unsafe folder %q accepted as %q", tc.folder, got)
		}
	}
}

func TestValidCloudinarySecureURLRejectsExternalAndMismatchedAssets(t *testing.T) {
	publicID := "school-a/applications/app-1/file-1"
	if !validCloudinarySecureURL("https://res.cloudinary.com/demo/raw/upload/v1/"+publicID, publicID) {
		t.Fatal("expected matching Cloudinary URL")
	}
	for _, rawURL := range []string{
		"http://res.cloudinary.com/demo/raw/upload/v1/" + publicID,
		"https://evil.example/" + publicID,
		"https://res.cloudinary.com/demo/raw/upload/v1/school-b/file-1",
		"https://attacker@res.cloudinary.com/demo/raw/upload/v1/" + publicID,
	} {
		if validCloudinarySecureURL(rawURL, publicID) {
			t.Fatalf("unsafe URL accepted: %s", rawURL)
		}
	}
}

func TestParseWebhookPayloadSupportsNestedTenantPathAndRejectsTraversal(t *testing.T) {
	payload, err := parseWebhookPayload([]byte("{\"public_id\":\"school-a/applications/app-1/file-1\",\"bytes\":42}"))
	if err != nil || payload.TenantID != "school-a" || payload.FileID != "file-1" {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}
	if _, err := parseWebhookPayload([]byte("{\"public_id\":\"school-a/../file-1\"}")); err == nil {
		t.Fatal("webhook traversal public_id accepted")
	}
}
