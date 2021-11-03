// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0
package uploaders

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/opencontainers/go-digest"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/gardener/component-cli/ociclient"
	"github.com/gardener/component-cli/pkg/transport/process"
	"github.com/gardener/component-cli/pkg/transport/process/utils"
)

type localOCIBlobUploader struct {
	client    ociclient.Client
	targetCtx cdv2.OCIRegistryRepository
}

func NewLocalOCIBlobUploader(client ociclient.Client, targetCtx cdv2.OCIRegistryRepository) (process.ResourceStreamProcessor, error) {
	if client == nil {
		return nil, errors.New("client must not be nil")
	}

	obj := localOCIBlobUploader{
		targetCtx: targetCtx,
		client:    client,
	}
	return &obj, nil
}

func (d *localOCIBlobUploader) Process(ctx context.Context, r io.Reader, w io.Writer) error {
	cd, res, blobreader, err := utils.ReadProcessorMessage(r)
	if err != nil {
		return fmt.Errorf("unable to read processor message: %w", err)
	}
	defer blobreader.Close()

	if res.Access.GetType() != cdv2.LocalOCIBlobType {
		return fmt.Errorf("unsupported access type: %s", res.Access.Type)
	}

	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer tmpfile.Close()

	_, err = io.Copy(tmpfile, blobreader)
	if err != nil {
		return err
	}

	_, err = tmpfile.Seek(0, 0)
	if err != nil {
		return err
	}

	fstat, err := tmpfile.Stat()
	if err != nil {
		return err
	}

	dgst, err := digest.FromReader(tmpfile)
	if err != nil {
		return err
	}

	_, err = tmpfile.Seek(0, 0)
	if err != nil {
		return err
	}

	desc := ocispecv1.Descriptor{
		Digest:    dgst,
		Size:      fstat.Size(),
		MediaType: res.Type,
	}

	err = d.uploadLocalOCIBlob(ctx, cd, res, tmpfile, desc)
	if err != nil {
		return fmt.Errorf("unable to upload blob: %w", err)
	}

	_, err = tmpfile.Seek(0, 0)
	if err != nil {
		return err
	}

	err = utils.WriteProcessorMessage(*cd, res, tmpfile, w)
	if err != nil {
		return fmt.Errorf("unable to write processor message: %w", err)
	}

	return nil
}

func (d *localOCIBlobUploader) uploadLocalOCIBlob(ctx context.Context, cd *cdv2.ComponentDescriptor, res cdv2.Resource, r io.Reader, desc ocispecv1.Descriptor) error {
	targetRef := createUploadRef(d.targetCtx, cd.Name, cd.Version)

	store := ociclient.GenericStore(func(ctx context.Context, desc ocispecv1.Descriptor, writer io.Writer) error {
		_, err := io.Copy(writer, r)
		return err
	})

	err := d.client.PushBlob(ctx, targetRef, desc, ociclient.WithStore(store))
	if err != nil {
		return fmt.Errorf("unable to push blob: %w", err)
	}

	return nil
}
