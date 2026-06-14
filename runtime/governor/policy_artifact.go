package governor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	policyArtifactSchemaVersion = "governor-policy-artifact.v0"
	policyArtifactKind          = "governor-policy"
	policyArtifactModePaper     = "paper"
	unknownFieldMatchParts      = 2
)

type canonicalPolicyArtifactV0 struct {
	SchemaVersion string
	ArtifactKind  string
	Mode          string
	Policy        Policy
	CanonicalJSON []byte
	Hash          string
}

type rawPolicyArtifactV0 struct {
	Mode                 string   `json:"mode"`
	AllowedActionKinds   []string `json:"allowedActionKinds"`
	MinimumQuality       string   `json:"minimumQuality"`
	MaximumApprovedCount *int     `json:"maximumApprovedCount"`
}

type marshaledPolicyArtifactV0 struct {
	SchemaVersion string                          `json:"schemaVersion"`
	ArtifactKind  string                          `json:"artifactKind"`
	Mode          string                          `json:"mode"`
	Policy        marshaledGovernorPolicyFieldsV0 `json:"policy"`
}

type marshaledGovernorPolicyFieldsV0 struct {
	AllowedActionKinds   []string `json:"allowedActionKinds"`
	MinimumQuality       string   `json:"minimumQuality"`
	MaximumApprovedCount int      `json:"maximumApprovedCount"`
}

var unknownFieldPattern = regexp.MustCompile(`^json: unknown field "([^"]+)"$`)

func canonicalizePolicyArtifactV0(raw []byte) (canonicalPolicyArtifactV0, error) {
	if len(raw) == 0 {
		return canonicalPolicyArtifactV0{}, validationError("governor policy payload is required")
	}

	var payload rawPolicyArtifactV0
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		if fieldName, ok := unknownFieldName(err); ok {
			return canonicalPolicyArtifactV0{}, validationError(
				fmt.Sprintf("unsupported field %q", fieldName),
			)
		}

		return canonicalPolicyArtifactV0{}, validationError(
			fmt.Sprintf("decode governor policy payload: %s", err.Error()),
		)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return canonicalPolicyArtifactV0{}, validationError(
			"decode governor policy payload: unexpected trailing content",
		)
	}

	mode := policyArtifactMode(payload.Mode)
	if mode != policyArtifactModePaper {
		return canonicalPolicyArtifactV0{}, validationError("only paper mode is supported")
	}

	allowedActionKinds := make([]domain.CandidateActionKind, 0, len(payload.AllowedActionKinds))
	for _, actionKind := range payload.AllowedActionKinds {
		normalizedActionKind, err := domain.NewCandidateActionKind(actionKind)
		if err != nil {
			return canonicalPolicyArtifactV0{}, validationError(
				fmt.Sprintf("unsupported allowed action kind %q", actionKind),
			)
		}

		allowedActionKinds = append(allowedActionKinds, normalizedActionKind)
	}

	minimumQuality, err := domain.NewDataQuality(payload.MinimumQuality)
	if err != nil {
		return canonicalPolicyArtifactV0{}, validationError(err.Error())
	}
	if payload.MaximumApprovedCount == nil {
		return canonicalPolicyArtifactV0{}, validationError("maximum approved action count is required")
	}

	policy := Policy{
		AllowedActionKinds:   allowedActionKinds,
		MinimumQuality:       minimumQuality,
		MaximumApprovedCount: *payload.MaximumApprovedCount,
	}
	canonical, err := canonicalizePolicy(policy)
	if err != nil {
		return canonicalPolicyArtifactV0{}, err
	}

	canonicalPolicy := Policy{
		AllowedActionKinds:   sortedAllowedActionKinds(canonical.allowedActionKinds),
		MinimumQuality:       minimumQuality,
		MaximumApprovedCount: canonical.maximumApprovedCount,
	}

	marshaled := marshaledPolicyArtifactV0{
		SchemaVersion: policyArtifactSchemaVersion,
		ArtifactKind:  policyArtifactKind,
		Mode:          mode,
		Policy: marshaledGovernorPolicyFieldsV0{
			AllowedActionKinds:   candidateActionKindsToStrings(canonicalPolicy.AllowedActionKinds),
			MinimumQuality:       canonicalPolicy.MinimumQuality.String(),
			MaximumApprovedCount: canonicalPolicy.MaximumApprovedCount,
		},
	}

	canonicalJSON, err := json.Marshal(marshaled)
	if err != nil {
		return canonicalPolicyArtifactV0{}, fmt.Errorf(
			"marshal governor policy artifact canonical payload: %w",
			err,
		)
	}

	hash := sha256.Sum256(canonicalJSON)

	return canonicalPolicyArtifactV0{
		SchemaVersion: marshaled.SchemaVersion,
		ArtifactKind:  marshaled.ArtifactKind,
		Mode:          marshaled.Mode,
		Policy:        canonicalPolicy,
		CanonicalJSON: append([]byte(nil), canonicalJSON...),
		Hash:          hex.EncodeToString(hash[:]),
	}, nil
}

func policyArtifactMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func sortedAllowedActionKinds(
	allowedActionKinds map[domain.CandidateActionKind]struct{},
) []domain.CandidateActionKind {
	canonical := make([]domain.CandidateActionKind, 0, len(allowedActionKinds))
	for actionKind := range allowedActionKinds {
		canonical = append(canonical, actionKind)
	}

	slices.SortFunc(canonical, func(left, right domain.CandidateActionKind) int {
		return stringsCompare(left.String(), right.String())
	})

	return canonical
}

func candidateActionKindsToStrings(
	actionKinds []domain.CandidateActionKind,
) []string {
	canonical := make([]string, 0, len(actionKinds))
	for _, actionKind := range actionKinds {
		canonical = append(canonical, actionKind.String())
	}

	return canonical
}

func unknownFieldName(err error) (string, bool) {
	matches := unknownFieldPattern.FindStringSubmatch(err.Error())
	if len(matches) != unknownFieldMatchParts {
		return "", false
	}

	return matches[1], true
}

func stringsCompare(left string, right string) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}

	return 0
}
