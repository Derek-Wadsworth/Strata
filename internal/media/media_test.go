package media

import "testing"

func TestIsImagePath(t *testing.T) {
	if !IsImagePath(`C:\tmp\lake.png`) {
		t.Fatal("png")
	}
	if !IsImagePath(`photo.JPEG`) {
		t.Fatal("jpeg")
	}
	if IsImagePath(`main.go`) {
		t.Fatal("go should not be image")
	}
}

func TestIsImageData(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0}
	if !IsImageData(png) {
		t.Fatal("png magic")
	}
	if IsImageData([]byte("hello")) {
		t.Fatal("text")
	}
}
