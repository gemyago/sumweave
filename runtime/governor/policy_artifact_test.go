package governor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestPolicyArtifact(t *testing.T) {
	t.Parallel()

	newFake := func(t *testing.T) faker.Faker {
		t.Helper()

		hasher := fnv.New64a()
		_, _ = hasher.Write([]byte(t.Name()))

		return faker.NewWithSeedInt64(int64(hasher.Sum64()))
	}

	randomWord := func(t *testing.T, fake faker.Faker, prefix string) string {
		t.Helper()

		return prefix + "-" + strings.ToLower(fake.Lorem().Word()) + "-" + strconv.Itoa(fake.IntBetween(1000, 9999))
	}

	marshalAllowedActionKinds := func(actionKinds []string) string {
		quoted := make([]string, 0, len(actionKinds))
		for _, actionKind := range actionKinds {
			quoted = append(quoted, strconv.Quote(actionKind))
		}

		return "[" + strings.Join(quoted, ",") + "]"
	}

	makeRawPayload := func(
		mode string,
		allowedActionKinds []string,
		minimumQuality string,
		maximumApprovedCount int,
		extraFields string,
	) []byte {
		return fmt.Appendf(
			nil,
			`{"mode":%q,"allowedActionKinds":%s,"minimumQuality":%q,"maximumApprovedCount":%d%s}`,
			mode,
			marshalAllowedActionKinds(allowedActionKinds),
			minimumQuality,
			maximumApprovedCount,
			extraFields,
		)
	}

	makeExpectedCanonicalJSON := func(
		minimumQuality string,
		maximumApprovedCount int,
	) []byte {
		return fmt.Appendf(
			nil,
			`{"schemaVersion":%q,"artifactKind":%q,"mode":%q,"policy":{"allowedActionKinds":[%q,%q],"minimumQuality":%q,"maximumApprovedCount":%d}}`,
			policyArtifactSchemaVersion,
			policyArtifactKind,
			policyArtifactModePaper,
			domain.CandidateActionKindLong.String(),
			domain.CandidateActionKindShort.String(),
			minimumQuality,
			maximumApprovedCount,
		)
	}

	t.Run("canonicalizePolicyArtifactV0", func(t *testing.T) {
		t.Parallel()

		t.Run("maps valid paper policy to governor policy and canonical hash", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			maximumApprovedCount := fake.IntBetween(0, 5)

			artifact, err := canonicalizePolicyArtifactV0(makeRawPayload(
				"  PAPER  ",
				[]string{" SHORT ", " long "},
				"  VALIDATED  ",
				maximumApprovedCount,
				"",
			))

			require.NoError(t, err)
			require.Equal(t, policyArtifactSchemaVersion, artifact.SchemaVersion)
			require.Equal(t, policyArtifactKind, artifact.ArtifactKind)
			require.Equal(t, policyArtifactModePaper, artifact.Mode)
			require.Equal(t, Policy{
				AllowedActionKinds: []domain.CandidateActionKind{
					domain.CandidateActionKindLong,
					domain.CandidateActionKindShort,
				},
				MinimumQuality:       domain.DataQualityValidated,
				MaximumApprovedCount: maximumApprovedCount,
			}, artifact.Policy)
			require.Equal(t, makeExpectedCanonicalJSON(
				domain.DataQualityValidated.String(),
				maximumApprovedCount,
			), artifact.CanonicalJSON)

			hash := sha256.Sum256(artifact.CanonicalJSON)
			require.Equal(t, hex.EncodeToString(hash[:]), artifact.Hash)
			require.Regexp(t, `^[0-9a-f]{64}$`, artifact.Hash)
		})

		t.Run("reordered allowed action kinds canonicalize identically", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			maximumApprovedCount := fake.IntBetween(0, 5)

			artifactA, err := canonicalizePolicyArtifactV0(makeRawPayload(
				policyArtifactModePaper,
				[]string{domain.CandidateActionKindLong.String(), domain.CandidateActionKindShort.String()},
				domain.DataQualityRaw.String(),
				maximumApprovedCount,
				"",
			))
			require.NoError(t, err)

			artifactB, err := canonicalizePolicyArtifactV0(makeRawPayload(
				" paper ",
				[]string{" SHORT ", " long "},
				" RAW ",
				maximumApprovedCount,
				"",
			))
			require.NoError(t, err)

			require.Equal(t, artifactA.Policy, artifactB.Policy)
			require.Equal(t, artifactA.CanonicalJSON, artifactB.CanonicalJSON)
			require.Equal(t, artifactA.Hash, artifactB.Hash)
		})

		t.Run("rejects invalid policy payloads", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)

			testCases := []struct {
				name        string
				payload     []byte
				expectedMsg string
			}{
				{
					name: "empty allowed action kinds",
					payload: makeRawPayload(
						policyArtifactModePaper,
						[]string{},
						domain.DataQualityRaw.String(),
						fake.IntBetween(0, 5),
						"",
					),
					expectedMsg: "allowed action kinds are required",
				},
				{
					name: "unsupported allowed action kind",
					payload: makeRawPayload(
						policyArtifactModePaper,
						[]string{randomWord(t, fake, "action")},
						domain.DataQualityRaw.String(),
						fake.IntBetween(0, 5),
						"",
					),
					expectedMsg: "unsupported allowed action kind",
				},
				{
					name: "unsupported minimum quality",
					payload: makeRawPayload(
						policyArtifactModePaper,
						[]string{domain.CandidateActionKindLong.String()},
						domain.DataQualitySuspect.String(),
						fake.IntBetween(0, 5),
						"",
					),
					expectedMsg: "unsupported minimum quality",
				},
				{
					name: "negative maximum approved action count",
					payload: makeRawPayload(
						policyArtifactModePaper,
						[]string{domain.CandidateActionKindLong.String()},
						domain.DataQualityRaw.String(),
						-fake.IntBetween(1, 5),
						"",
					),
					expectedMsg: "maximum approved action count must be zero or greater",
				},
				{
					name: "missing maximum approved action count",
					payload: fmt.Appendf(nil,
						`{"mode":%q,"allowedActionKinds":[%q],"minimumQuality":%q}`,
						policyArtifactModePaper,
						domain.CandidateActionKindLong.String(),
						domain.DataQualityRaw.String(),
					),
					expectedMsg: "maximum approved action count is required",
				},
				{
					name: "null maximum approved action count",
					payload: fmt.Appendf(nil,
						`{"mode":%q,"allowedActionKinds":[%q],"minimumQuality":%q,"maximumApprovedCount":null}`,
						policyArtifactModePaper,
						domain.CandidateActionKindLong.String(),
						domain.DataQualityRaw.String(),
					),
					expectedMsg: "maximum approved action count is required",
				},
				{
					name: "non-paper mode",
					payload: makeRawPayload(
						"live",
						[]string{domain.CandidateActionKindLong.String()},
						domain.DataQualityRaw.String(),
						fake.IntBetween(0, 5),
						"",
					),
					expectedMsg: "only paper mode is supported",
				},
				{
					name: "live-routing field",
					payload: makeRawPayload(
						policyArtifactModePaper,
						[]string{domain.CandidateActionKindLong.String()},
						domain.DataQualityRaw.String(),
						fake.IntBetween(0, 5),
						`,"privateEndpoint":"https://`+randomWord(t, fake, "endpoint")+`.example.com"`,
					),
					expectedMsg: "unsupported field \"privateEndpoint\"",
				},
				{
					name: "credential-like field",
					payload: makeRawPayload(
						policyArtifactModePaper,
						[]string{domain.CandidateActionKindLong.String()},
						domain.DataQualityRaw.String(),
						fake.IntBetween(0, 5),
						`,"apiKey":"`+randomWord(t, fake, "api-key")+`"`,
					),
					expectedMsg: "unsupported field \"apiKey\"",
				},
				{
					name: "trailing content after valid payload",
					payload: append(
						makeRawPayload(
							policyArtifactModePaper,
							[]string{domain.CandidateActionKindLong.String()},
							domain.DataQualityRaw.String(),
							fake.IntBetween(0, 5),
							"",
						),
						[]byte(randomWord(t, fake, "trailing"))...,
					),
					expectedMsg: "decode governor policy payload: unexpected trailing content",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					artifact, err := canonicalizePolicyArtifactV0(testCase.payload)

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.expectedMsg)
					require.Equal(t, canonicalPolicyArtifactV0{}, artifact)
				})
			}
		})
	})
}
