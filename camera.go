package meterman

import (
    "github.com/aamcrae/webcam"
    "fmt"
    "image"
    "image/color"
)

type Camera struct {
    cam *webcam.Webcam
    Width int
    Height int
    Format string
    Timeout uint32
}

type Format int
const (
    YUYV Format = iota
    MJPEG
    maxFormats
)

var formatMap = map[Format]string {
    YUYV    : "YUYV 4:2:2",
    MJPEG   : "Motion-JPEF",
}

func OpenCamera(name string) (*Camera, error) {
	c, err := webcam.Open("/dev/video0")
	if err != nil {
        return nil, err
	}
    return &Camera{cam:c, Timeout:5}, nil
}

func (c *Camera) Close() {
    c.cam.StopStreaming()
    c.cam.Close()
}

func (c *Camera) Init(format Format, resolution string) error {
    // Get the supported formats and their descriptions.
	format_desc := c.cam.GetSupportedFormats()
    descToFormat := make(map[string]webcam.PixelFormat)
    for k, v := range format_desc {
        descToFormat[v] = k
    }
    // Translate the requested
    desc, ok := formatMap[format]
    if !ok {
        return fmt.Errorf("Unknown format: %d", format)
    }
	f, ok := descToFormat[desc]
    if !ok {
        return fmt.Errorf("Camera does not support this format: %d", format)
    }

    // Build a map of resolution names from the description.
    sizeMap := make(map[string]webcam.FrameSize)
    for _, value := range c.cam.GetSupportedFrameSizes(f) {
        sizeMap[value.GetString()] = value
    }

	sz, ok := sizeMap[resolution]
    if !ok {
        return fmt.Errorf("Unsupported resoluton: %s", resolution)
    }

	_, w, h, err := c.cam.SetImageFormat(f, uint32(sz.MaxWidth), uint32(sz.MaxHeight))

	if err != nil {
        return err
	}
    c.Width = int(w)
    c.Height = int(h)

    if err := c.cam.SetBufferCount(2); err != nil {
		return err
    }
    c.cam.SetAutoWhiteBalance(true)
	return c.cam.StartStreaming()
}

func (c *Camera) GetFrame() (*image.RGBA, error) {
    for {
	    err := c.cam.WaitForFrame(c.Timeout)

	    switch err.(type) {
	    case nil:
	    case *webcam.Timeout:
		    continue
	    default:
                return nil, err
	    }

	    frame, index, err := c.cam.GetFrame()
        if err != nil {
            return nil, err
        }
        defer c.cam.ReleaseFrame(index)
        expLen := 2 * c.Width * c.Height
	    if len(frame) != expLen {
            return nil, fmt.Errorf("Wrong frame length (exp: %d, read %d)", expLen, len(frame))
        }
        return c.convert(frame), nil
    }
}

// Convert frame buffer to RGB image.
func (c *Camera) convert(frame []byte) *image.RGBA {
    img := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))
    for y := 0; y < c.Height; y++ {
        for x := 0; x < c.Width; x += 2 {
            pix := frame[c.Width * y * 2 + x * 2:]
            img.Set(x, y, color.YCbCr{pix[0], pix[1], pix[3]})
            img.Set(x + 1, y, color.YCbCr{pix[2], pix[1], pix[3]})
        }
    }
    return img
}
