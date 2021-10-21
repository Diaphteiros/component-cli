// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier
package downloaders

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	cdoci "github.com/gardener/component-spec/bindings-go/oci"

	"github.com/gardener/component-cli/ociclient"
	"github.com/gardener/component-cli/pkg/transport/process"
)

type localOCIBlobDownloader struct {
	client ociclient.Client
}

func NewLocalOCIBlobDownloader(client ociclient.Client) process.ResourceStreamProcessor {
	obj := localOCIBlobDownloader{
		client: client,
	}
	return &obj
}

func (d *localOCIBlobDownloader) Process(ctx context.Context, r io.Reader, w io.Writer) error {
	cd, res, _, err := process.ReadProcessorMessage(r)
	if err != nil {
		return fmt.Errorf("unable to read input archive: %w", err)
	}

	if res.Access.GetType() != cdv2.LocalOCIBlobType {
		return fmt.Errorf("unsupported access type: %+v", res.Access)
	}

	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		return fmt.Errorf("unable to create tempfile: %w", err)
	}
	defer tmpfile.Close()

	if err := d.fetchLocalOCIBlob(ctx, cd, res, tmpfile); err != nil {
		return fmt.Errorf("unable to fetch blob: %w", err)
	}

	if _, err := tmpfile.Seek(0, 0); err != nil {
		return fmt.Errorf("unable to seek to beginning of tempfile: %w", err)
	}

	if err := process.WriteProcessorMessage(*cd, res, tmpfile, w); err != nil {
		return fmt.Errorf("unable to write processor message: %w", err)
	}

	return nil
}

func (d *localOCIBlobDownloader) fetchLocalOCIBlob(ctx context.Context, cd *cdv2.ComponentDescriptor, res cdv2.Resource, w io.Writer) error {
	repoctx := cdv2.OCIRegistryRepository{}
	if err := cd.GetEffectiveRepositoryContext().DecodeInto(&repoctx); err != nil {
		return fmt.Errorf("unable to decode repository context: %w", err)
	}

	resolver := cdoci.NewResolver(d.client)
	_, blobResolver, err := resolver.ResolveWithBlobResolver(ctx, &repoctx, cd.Name, cd.Version)
	if err != nil {
		return fmt.Errorf("unable to resolve component descriptor: %w", err)
	}

	if _, err := blobResolver.Resolve(ctx, res, w); err != nil {
		return fmt.Errorf("unable to to resolve blob: %w", err)
	}

	return nil
}
