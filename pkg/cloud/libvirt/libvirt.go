//go:build cgo

package libvirt

import (
	"fmt"
	"io"

	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/olog"

	lv "libvirt.org/go/libvirt"
)

var _ = cloud.Uploader(&libvirtUploader{})

type libvirtUploader struct {
	connection string
	pool       string
	volume     string
}

func NewUploader(connection string, pool string, volume string) (cloud.Uploader, error) {
	return &libvirtUploader{
		connection: connection,
		pool:       pool,
		volume:     volume,
	}, nil
}

func (lu *libvirtUploader) Check(status io.Writer) error {
	return nil
}

func (lu *libvirtUploader) UploadAndRegister(r io.Reader, uploadSize uint64, status io.Writer) (err error) {
	fmt.Fprintf(status, "Uploading to libvirt...\n")

	conn, err := lv.NewConnect(lu.connection)
	if err != nil {
		return fmt.Errorf("Failed to connect to libvirt: %w", err)
	}
	defer conn.Close()

	pool, err := conn.LookupStoragePoolByName(lu.pool)
	if err != nil {
		return fmt.Errorf("Failed to find storage pool: %w", err)
	}

	defer func() {
		if err := pool.Free(); err != nil {
			olog.Printf("Failed to free pool: %v", err)
		}
	}()

	volXML := lu.VolumeXML(lu.volume, uploadSize)
	vol, err := pool.StorageVolCreateXML(volXML, 0)
	if err != nil {
		return fmt.Errorf("Failed to create a libvirt volume: %w", err)
	}
	defer func() {
		if err := vol.Free(); err != nil {
			olog.Printf("Failed to free volume: %v", err)
		}
	}()

	err = lu.Upload(conn, vol, r, uploadSize)
	if err != nil {
		return fmt.Errorf("Failed to upload the file to libvirt: %w", err)
	}

	return nil
}

func (lu *libvirtUploader) VolumeXML(name string, size uint64) string {
	return fmt.Sprintf(`
<volume>
  <name>%s</name>
  <capacity unit="bytes">%d</capacity>
  <target>
	<format type="qcow2"/>
  </target>
</volume>`, name, size)
}

func (lu *libvirtUploader) Upload(conn *lv.Connect, vol *lv.StorageVol, r io.Reader, size uint64) (err error) {
	stream, err := conn.NewStream(lv.STREAM_NONBLOCK)
	if err != nil {
		return fmt.Errorf("Failed to initialize an upload stream: %w", err)
	}
	defer func() {
		if err := stream.Free(); err != nil {
			olog.Printf("Failed to free stream: %v", err)
		}
	}()

	if err := vol.Upload(stream, 0, size, 0); err != nil {
		return fmt.Errorf("Failed to start the upload: %w", err)
	}

	buf := make([]byte, 64*1024)
	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("Failed to read the file: %w", err)
		}
		if n > 0 {
			if _, sendErr := stream.Send(buf[:n]); sendErr != nil {
				return fmt.Errorf("Failed to stream the buffer: %w", sendErr)
			}
		}
	}

	if err := stream.Finish(); err != nil {
		return fmt.Errorf("Failed to finish stream: %w", err)
	}

	return nil
}
