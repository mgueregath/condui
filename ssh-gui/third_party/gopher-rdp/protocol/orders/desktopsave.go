package orders

// DesktopSaveCache holds a linear byte buffer for DesktopSave/SaveBitmap orders.
// The server stores/restores rectangular framebuffer regions at byte offsets.
// Size matches the desktopSaveSize advertised in the Order capability set.
type DesktopSaveCache struct {
	data [desktopSaveCacheSize]byte
}

// desktopSaveCacheSize = 0x38400 * 4 = 921600 bytes (MS-RDPBCGR 2.2.7.1.5).
// This corresponds to desktopSaveSize=230400 (in pixels) * 4 bytes/pixel.
const desktopSaveCacheSize = 230400 * 4

// Save copies a rectangle from the framebuffer (top-down RGBA) into the cache.
// offset is in pixel units (multiplied by 4 internally).
// fb is the framebuffer pixels, fbW/fbH the framebuffer dimensions.
// The framebuffer is bottom-up BGRX with stride = fbW*4.
func (dc *DesktopSaveCache) Save(offset uint32, left, top, right, bottom int, fb []byte, fbW, fbH int) {
	w := right - left + 1
	h := bottom - top + 1
	if w <= 0 || h <= 0 {
		return
	}

	byteOff := int(offset) * 4
	need := w * h * 4
	if byteOff+need > len(dc.data) {
		return
	}

	fbStride := fbW * 4
	idx := byteOff
	for dy := 0; dy < h; dy++ {
		// framebuffer is bottom-up: logical row (top+dy) → buffer row (fbH-1-(top+dy))
		fbRow := fbH - 1 - (top + dy)
		if fbRow < 0 || fbRow >= fbH {
			idx += w * 4
			continue
		}
		for dx := 0; dx < w; dx++ {
			px := left + dx
			if px < 0 || px >= fbW {
				idx += 4
				continue
			}
			fi := fbRow*fbStride + px*4
			dc.data[idx] = fb[fi]
			dc.data[idx+1] = fb[fi+1]
			dc.data[idx+2] = fb[fi+2]
			dc.data[idx+3] = fb[fi+3]
			idx += 4
		}
	}
}

// Restore copies a rectangle from the cache back into the framebuffer.
// Returns the bounding rectangle (x, y, w, h) of the restored area.
func (dc *DesktopSaveCache) Restore(offset uint32, left, top, right, bottom int, fb []byte, fbW, fbH int) (x, y, w, h int) {
	w = right - left + 1
	h = bottom - top + 1
	if w <= 0 || h <= 0 {
		return 0, 0, 0, 0
	}

	byteOff := int(offset) * 4
	need := w * h * 4
	if byteOff+need > len(dc.data) {
		return 0, 0, 0, 0
	}

	fbStride := fbW * 4
	idx := byteOff
	for dy := 0; dy < h; dy++ {
		fbRow := fbH - 1 - (top + dy)
		if fbRow < 0 || fbRow >= fbH {
			idx += w * 4
			continue
		}
		for dx := 0; dx < w; dx++ {
			px := left + dx
			if px < 0 || px >= fbW {
				idx += 4
				continue
			}
			fi := fbRow*fbStride + px*4
			fb[fi] = dc.data[idx]
			fb[fi+1] = dc.data[idx+1]
			fb[fi+2] = dc.data[idx+2]
			fb[fi+3] = dc.data[idx+3]
			idx += 4
		}
	}

	return left, top, w, h
}
