package meterman

import (
    "github.com/aamcrae/webcam"
    "fmt"
)

type Camera struct {
    cam *webcam.Webcam
    Width int
    Height int
    Format string
    Timeout uint32
    newFrame func(int, int, []byte, func()) (Frame, error)
}

func OpenCamera(name string) (*Camera, error) {
	c, err := webcam.Open(name)
	if err != nil {
        return nil, err
	}
    return &Camera{cam:c, Timeout:5}, nil
}

func (c *Camera) Close() {
    c.cam.StopStreaming()
    c.cam.Close()
}

func (c *Camera) Init(format string, resolution string) error {
    // Get the supported formats and their descriptions.
	format_desc := c.cam.GetSupportedFormats()
    var pixelFormat webcam.PixelFormat
    var found bool
    for k, v := range format_desc {
        if v == format {
            found = true
            pixelFormat = k
            break
        }
    }
    if !found {
        return fmt.Errorf("Camera does not support this format: %s", format)
    }
    var err error
    if c.newFrame, err = GetFramer(format); err != nil {
        return err
    }

    // Build a map of resolution names from the description.
    sizeMap := make(map[string]webcam.FrameSize)
    for _, value := range c.cam.GetSupportedFrameSizes(pixelFormat) {
        sizeMap[value.GetString()] = value
    }

	sz, ok := sizeMap[resolution]
    if !ok {
        return fmt.Errorf("Unsupported resoluton: %s", resolution)
    }

	_, w, h, err := c.cam.SetImageFormat(pixelFormat, uint32(sz.MaxWidth), uint32(sz.MaxHeight))

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

func (c *Camera) GetFrame() (Frame, error) {
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
        rel := func() {
            c.cam.ReleaseFrame(index)
        }
        return c.newFrame(c.Width, c.Height, frame, rel)
    }
}

// Return map of supported formats and resolutions.
func (c *Camera) Query() map[string][]string {
    m := map[string][]string{}
	formats := c.cam.GetSupportedFormats()
    for f, fs := range formats {
        r := []string{}
        for _, value := range c.cam.GetSupportedFrameSizes(f) {
            if value.StepWidth == 0 && value.StepHeight == 0 {
                r = append(r, fmt.Sprintf("%dx%d", value.MaxWidth, value.MaxHeight))
            }
        }
        m[fs] = r
    }
    return m
}
