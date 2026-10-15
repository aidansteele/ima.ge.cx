package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/converter"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/stargz-snapshotter/estargz"
	conv "github.com/containerd/stargz-snapshotter/nativeconverter/estargz"
	"github.com/containerd/stargz-snapshotter/util/testutil"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"os"
	"testing"
)

func TestIndex(t *testing.T) {
	//ctx := context.Background()

	ref, err := name.ParseReference("ubuntu:20.04")
	if err != nil {
		panic(err)
	}

	//idx, err := remote.Index(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	//if err != nil {
	//	panic(err)
	//}

	repository := ref.Context()
	//list, err := remote.Catalog(ctx, repository.Registry)
	list, err := remote.List(repository)
	if err != nil {
		panic(err)
	}

	spew.Dump(list)
}

func TestWipe(t *testing.T) {
	// This deletes every item in $TABLE, so never run it as part of `go test ./...`.
	if os.Getenv("WIPE_TABLE") != "1" {
		t.Skip("set WIPE_TABLE=1 to delete every item in $TABLE")
	}

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	dynamo := dynamodb.NewFromConfig(cfg)
	table := os.Getenv("TABLE")

	p := dynamodb.NewScanPaginator(dynamo, &dynamodb.ScanInput{TableName: &table})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			panic(err)
		}

		for _, item := range page.Items {
			fmt.Println(item["pk"], item["sk"])
			//if item["pk"].(*types.AttributeValueMemberS).Value != "image#index.docker.io/mesosphere/whiteout" {
			//	continue
			//}

			key := map[string]types.AttributeValue{"pk": item["pk"], "sk": item["sk"]}
			_, err = dynamo.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: &table, Key: key})
			if err != nil {
				panic(err)
			}
		}
	}
}

func TestMy(t *testing.T) {
	ctx := context.Background()
	desc, cs, err := testutil.EnsureHello(ctx)
	if err != nil {
		t.Fatal(err)
	}

	lcf := conv.LayerConvertFunc(estargz.WithPrioritizedFiles([]string{"hello"}))
	docker2oci := true
	platformMC := platforms.DefaultStrict()
	//platformMC := platforms.MustParse("linux/amd64")
	cf := converter.DefaultIndexConvertFunc(lcf, docker2oci, platformMC)

	newDesc, err := cf(ctx, cs, *desc)
	if err != nil {
		t.Fatal(err)
	}

	var tocDigests []string
	handler := func(hCtx context.Context, hDesc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if hDesc.Annotations != nil {
			if x, ok := hDesc.Annotations[estargz.TOCJSONDigestAnnotation]; ok && len(x) > 0 {
				tocDigests = append(tocDigests, x)
			}
		}
		return nil, nil
	}
	handlers := images.Handlers(
		images.ChildrenHandler(cs),
		images.HandlerFunc(handler),
	)
	if err := images.Walk(ctx, handlers, *newDesc); err != nil {
		t.Fatal(err)
	}

	if len(tocDigests) == 0 {
		t.Fatal("no eStargz layer was created")
	}
}
