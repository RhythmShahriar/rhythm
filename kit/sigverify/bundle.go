// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package sigverify

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
)

// legacyBundle is the bundle cosign v2 sign-blob --bundle writes: a
// base64 PEM certificate, the signature and Rekor's signed entry timestamp.
// Releases signed before the move to cosign v3 carry this format.
type legacyBundle struct {
	Base64Signature string `json:"base64Signature"`
	Cert            string `json:"cert"`
	RekorBundle     *struct {
		SignedEntryTimestamp []byte `json:"SignedEntryTimestamp"`
		Payload              struct {
			Body           string `json:"body"`
			IntegratedTime int64  `json:"integratedTime"`
			LogIndex       int64  `json:"logIndex"`
			LogID          string `json:"logID"`
		} `json:"Payload"`
	} `json:"rekorBundle"`
}

// hashedRekord is the part of a Rekor hashedrekord entry body we read.
type hashedRekord struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Data struct {
			Hash struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"hash"`
		} `json:"data"`
	} `json:"spec"`
}

// parseBundle reads a Sigstore protobuf bundle or a cosign v2 bundle.
func parseBundle(data []byte) (*bundle.Bundle, error) {
	var probe struct {
		MediaType   string          `json:"mediaType"`
		RekorBundle json.RawMessage `json:"rekorBundle"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("bundle is not JSON: %w", err)
	}
	switch {
	case probe.MediaType != "":
		var b bundle.Bundle
		if err := b.UnmarshalJSON(data); err != nil {
			return nil, fmt.Errorf("reading Sigstore bundle: %w", err)
		}
		return &b, nil
	case probe.RekorBundle != nil:
		pb, err := convertLegacy(data)
		if err != nil {
			return nil, fmt.Errorf("reading cosign bundle: %w", err)
		}
		return bundle.NewBundle(pb)
	}
	return nil, errors.New("not a Sigstore or cosign bundle")
}

// convertLegacy turns a cosign v2 bundle into the equivalent v0.1 protobuf
// bundle (certificate chain plus inclusion promise), the same mapping
// cosign applies before verifying one. Every field still goes through
// sigstore-go's checks: the body must match the certificate, signature and
// digest. The signed entry timestamp must verify against Rekor's key.
func convertLegacy(data []byte) (*protobundle.Bundle, error) {
	var lb legacyBundle
	if err := json.Unmarshal(data, &lb); err != nil {
		return nil, err
	}
	if lb.RekorBundle == nil || len(lb.RekorBundle.SignedEntryTimestamp) == 0 {
		return nil, errors.New("no Rekor entry")
	}
	certPEM, err := base64.StdEncoding.DecodeString(lb.Cert)
	if err != nil {
		return nil, fmt.Errorf("certificate: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("certificate is not PEM")
	}
	sig, err := base64.StdEncoding.DecodeString(lb.Base64Signature)
	if err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}
	p := lb.RekorBundle.Payload
	body, err := base64.StdEncoding.DecodeString(p.Body)
	if err != nil {
		return nil, fmt.Errorf("log entry body: %w", err)
	}
	var entry hashedRekord
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, fmt.Errorf("log entry body: %w", err)
	}
	if entry.Kind != "hashedrekord" || entry.Spec.Data.Hash.Algorithm != "sha256" {
		return nil, fmt.Errorf("unsupported Rekor entry %s %s", entry.Kind, entry.Spec.Data.Hash.Algorithm)
	}
	digest, err := hex.DecodeString(entry.Spec.Data.Hash.Value)
	if err != nil {
		return nil, fmt.Errorf("log entry digest: %w", err)
	}
	logID, err := hex.DecodeString(p.LogID)
	if err != nil {
		return nil, fmt.Errorf("log ID: %w", err)
	}
	return &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.1",
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_X509CertificateChain{
				X509CertificateChain: &protocommon.X509CertificateChain{
					Certificates: []*protocommon.X509Certificate{{RawBytes: block.Bytes}},
				},
			},
			TlogEntries: []*protorekor.TransparencyLogEntry{{
				LogIndex:          p.LogIndex,
				LogId:             &protocommon.LogId{KeyId: logID},
				KindVersion:       &protorekor.KindVersion{Kind: entry.Kind, Version: entry.APIVersion},
				IntegratedTime:    p.IntegratedTime,
				InclusionPromise:  &protorekor.InclusionPromise{SignedEntryTimestamp: lb.RekorBundle.SignedEntryTimestamp},
				CanonicalizedBody: body,
			}},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    digest,
				},
				Signature: sig,
			},
		},
	}, nil
}
