/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package images

import (
	"testing"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func makeImage(name string, targetDigest digest.Digest) images.Image {
	return images.Image{
		Name: name,
		Target: ocispec.Descriptor{
			Digest: targetDigest,
		},
	}
}

func TestDeduplicateCRIImages(t *testing.T) {
	d1 := digest.FromString("image1")
	d2 := digest.FromString("image2")

	repoTag := makeImage("docker.io/library/nginx:1.25", d1)
	repoDigest := makeImage("docker.io/library/nginx@"+d1.String(), d1)
	configID := makeImage(d1.String(), d1)

	// A second image with only a digest ref (dangling — no tag)
	dangling := makeImage("docker.io/library/busybox@"+d2.String(), d2)

	tests := []struct {
		name     string
		input    []images.Image
		wantRefs []string
		wantLen  int
	}{
		{
			name:     "three CRI refs collapse to one tagged ref",
			input:    []images.Image{repoTag, repoDigest, configID},
			wantRefs: []string{"docker.io/library/nginx:1.25"},
		},
		{
			name:  "dangling image with no tag is preserved",
			input: []images.Image{repoDigest, configID, dangling},
			// repoDigest and configID share d1 but neither is tagged → one kept
			// dangling shares d2 → one kept
			wantLen: 2,
		},
		{
			name:     "tagged ref always wins over digest ref",
			input:    []images.Image{repoDigest, repoTag, configID},
			wantRefs: []string{"docker.io/library/nginx:1.25"},
		},
		{
			name:     "empty list returns empty",
			input:    []images.Image{},
			wantRefs: []string{},
		},
		{
			name:     "multiple distinct tagged images preserved",
			input:    []images.Image{repoTag, makeImage("docker.io/library/alpine:3.18", d2)},
			wantRefs: []string{"docker.io/library/nginx:1.25", "docker.io/library/alpine:3.18"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deduplicateCRIImages(tc.input)
			if tc.wantRefs != nil {
				if len(got) != len(tc.wantRefs) {
					t.Fatalf("got %d images, want %d: %v", len(got), len(tc.wantRefs), imageNames(got))
				}
				wantSet := make(map[string]struct{}, len(tc.wantRefs))
				for _, r := range tc.wantRefs {
					wantSet[r] = struct{}{}
				}
				for _, img := range got {
					if _, ok := wantSet[img.Name]; !ok {
						t.Errorf("unexpected ref in result: %s", img.Name)
					}
				}
			} else if tc.wantLen > 0 {
				if len(got) != tc.wantLen {
					t.Fatalf("got %d images, want %d: %v", len(got), tc.wantLen, imageNames(got))
				}
			}
		})
	}
}

func imageNames(imgs []images.Image) []string {
	names := make([]string, len(imgs))
	for i, img := range imgs {
		names[i] = img.Name
	}
	return names
}
