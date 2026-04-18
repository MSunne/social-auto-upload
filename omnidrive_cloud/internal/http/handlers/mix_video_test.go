package handlers

import "testing"

func TestMixVideoAssetExtensionsVideosOnly(t *testing.T) {
	if _, ok := mixVideoAssetExtensions[".mp4"]; !ok {
		t.Fatal("expected .mp4 to be allowed for mix video assets")
	}
	if _, ok := mixVideoAssetExtensions[".mov"]; !ok {
		t.Fatal("expected .mov to be allowed for mix video assets")
	}
	if _, ok := mixVideoAssetExtensions[".webm"]; !ok {
		t.Fatal("expected .webm to be allowed for mix video assets")
	}
	if _, ok := mixVideoAssetExtensions[".png"]; ok {
		t.Fatal("expected .png to be rejected for mix video assets")
	}
}

func TestValidateMixVideoMimeRejectsImageAsset(t *testing.T) {
	if _, err := validateMixVideoMime("poster.png", "image/png", []byte("png-bytes"), mixVideoAssetMIMEs); err == nil {
		t.Fatal("expected image asset mime to be rejected")
	}
}
