// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package sigverifytest is a throwaway Sigstore instance for tests: a
// Fulcio-style CA and a Rekor-style log whose keys live in memory, so a
// test can sign a checksums.txt the way GitHub Actions and cosign would
// and check it with a sigverify.Verifier built from CA.Options.
//
// The bundles it writes are real Sigstore protobuf bundles (version 0.1:
// a certificate chain, a hashedrekord log entry and its signed entry
// timestamp). Only a Verifier given this CA's TrustedMaterial accepts
// them; the public-good root never does. Nothing here touches the
// network.
package sigverifytest

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/RhythmShahriar/rhythm/kit/sigverify"
)

// CA is an in-memory Sigstore instance. It is safe for concurrent use.
type CA struct {
	rootCert, interCert *x509.Certificate
	interKey            *ecdsa.PrivateKey
	rekorKey            *ecdsa.PrivateKey
	logID               []byte // sha256 of the Rekor key, as Rekor names its log
	material            root.TrustedMaterial
}

// New makes a CA whose certificates and log are valid from an hour ago
// to an hour from now.
func New() (*CA, error) {
	now := time.Now()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "sigverifytest root", Organization: []string{"sigverifytest"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootCert, err := create(rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	interCert, err := create(&x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "sigverifytest intermediate", Organization: []string{"sigverifytest"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}, rootCert, &interKey.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	rekorKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	pub, err := x509.MarshalPKIXPublicKey(&rekorKey.PublicKey)
	if err != nil {
		return nil, err
	}
	id := sha256.Sum256(pub)
	ca := &CA{rootCert: rootCert, interCert: interCert, interKey: interKey, rekorKey: rekorKey, logID: id[:]}
	ca.material = &material{
		fulcio: &root.FulcioCertificateAuthority{
			Root:                rootCert,
			Intermediates:       []*x509.Certificate{interCert},
			ValidityPeriodStart: now.Add(-time.Hour),
			ValidityPeriodEnd:   now.Add(time.Hour),
			URI:                 "https://fulcio.sigverifytest.invalid",
		},
		rekor: map[string]*root.TransparencyLog{hex.EncodeToString(id[:]): {
			BaseURL:             "https://rekor.sigverifytest.invalid",
			ID:                  id[:],
			ValidityPeriodStart: now.Add(-time.Hour),
			ValidityPeriodEnd:   now.Add(time.Hour),
			HashFunc:            crypto.SHA256,
			PublicKey:           &rekorKey.PublicKey,
			SignatureHashFunc:   crypto.SHA256,
		}},
	}
	return ca, nil
}

// TrustedMaterial is the CA and log as a Sigstore trusted root.
func (c *CA) TrustedMaterial() root.TrustedMaterial { return c.material }

// Options returns sigverify options that trust this CA and nothing else.
func (c *CA) Options() sigverify.Options {
	return sigverify.Options{TrustedMaterial: c.material}
}

// Identity is who a signature claims to come from: what Fulcio would put
// in the certificate for a GitHub Actions run.
type Identity struct {
	// SAN is the workflow identity, such as
	// https://github.com/o/r/.github/workflows/release.yml@refs/tags/v1.0.0.
	SAN string
	// Issuer is the OIDC issuer; empty means GitHub Actions.
	Issuer string
	// Repo is the source repository as owner/name.
	Repo string
	// Ref is the source ref, such as refs/tags/v1.0.0.
	Ref string
}

// Shared is the identity of the shared Rhythm release workflow at master
// building repo's tag v<version>, as every Rhythm tool is signed.
func Shared(repo, version string) Identity {
	return Identity{SAN: sigverify.Workflow + "@" + sigverify.MasterRef, Repo: repo, Ref: "refs/tags/v" + version}
}

// Sign signs data (a checksums.txt) as id and returns the bundle JSON a
// release would publish as checksums.txt.sigstore.json.
func (c *CA) Sign(id Identity, data []byte) ([]byte, error) {
	if id.Issuer == "" {
		id.Issuer = sigverify.Issuer
	}
	san, err := url.Parse(id.SAN)
	if err != nil {
		return nil, fmt.Errorf("sigverifytest: SAN: %w", err)
	}
	exts, err := extensions(id)
	if err != nil {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	leaf, err := create(&x509.Certificate{
		SerialNumber:    big.NewInt(now.UnixNano()),
		NotBefore:       now.Add(-time.Minute),
		NotAfter:        now.Add(10 * time.Minute),
		KeyUsage:        x509.KeyUsageDigitalSignature,
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs:            []*url.URL{san},
		ExtraExtensions: exts,
	}, c.interCert, &key.PublicKey, c.interKey)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return nil, err
	}

	// The hashedrekord entry Rekor would log, in canonical JSON:
	// json.Marshal sorts map keys and writes no spaces.
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})
	body, err := json.Marshal(map[string]any{
		"apiVersion": "0.0.1",
		"kind":       "hashedrekord",
		"spec": map[string]any{
			"data": map[string]any{"hash": map[string]any{"algorithm": "sha256", "value": hex.EncodeToString(digest[:])}},
			"signature": map[string]any{
				"content":   base64.StdEncoding.EncodeToString(sig),
				"publicKey": map[string]any{"content": base64.StdEncoding.EncodeToString(certPEM)},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	const logIndex = 1000
	integrated := now.Unix()
	set, err := c.signEntry(body, integrated, logIndex)
	if err != nil {
		return nil, err
	}
	b := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.1",
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_X509CertificateChain{
				X509CertificateChain: &protocommon.X509CertificateChain{
					Certificates: []*protocommon.X509Certificate{{RawBytes: leaf.Raw}},
				},
			},
			TlogEntries: []*protorekor.TransparencyLogEntry{{
				LogIndex:          logIndex,
				LogId:             &protocommon.LogId{KeyId: c.logID},
				KindVersion:       &protorekor.KindVersion{Kind: "hashedrekord", Version: "0.0.1"},
				IntegratedTime:    integrated,
				InclusionPromise:  &protorekor.InclusionPromise{SignedEntryTimestamp: set},
				CanonicalizedBody: body,
			}},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{Algorithm: protocommon.HashAlgorithm_SHA2_256, Digest: digest[:]},
				Signature:     sig,
			},
		},
	}
	return protojson.Marshal(b)
}

// signEntry is the log's signed entry timestamp over one entry: the
// signature over the canonical JSON of body, time, index and log ID.
func (c *CA) signEntry(body []byte, integrated, index int64) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"body":           base64.StdEncoding.EncodeToString(body),
		"integratedTime": integrated,
		"logID":          hex.EncodeToString(c.logID),
		"logIndex":       index,
	})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(payload)
	return ecdsa.SignASN1(rand.Reader, c.rekorKey, digest[:])
}

// extensions are Fulcio's certificate extensions for id: the OIDC issuer
// and the source repository and ref, each a DER UTF8String.
func extensions(id Identity) ([]pkix.Extension, error) {
	var out []pkix.Extension
	for _, e := range []struct {
		oid   asn1.ObjectIdentifier
		value string
	}{
		{certificate.OIDIssuerV2, id.Issuer},
		{certificate.OIDSourceRepositoryURI, "https://github.com/" + id.Repo},
		{certificate.OIDSourceRepositoryRef, id.Ref},
	} {
		der, err := asn1.MarshalWithParams(e.value, "utf8")
		if err != nil {
			return nil, err
		}
		out = append(out, pkix.Extension{Id: e.oid, Value: der})
	}
	return out, nil
}

func create(tmpl, parent *x509.Certificate, pub *ecdsa.PublicKey, priv *ecdsa.PrivateKey) (*x509.Certificate, error) {
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, priv)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

// material is the CA and log as root.TrustedMaterial, with no timestamp
// authority and no CT log.
type material struct {
	root.BaseTrustedMaterial
	fulcio *root.FulcioCertificateAuthority
	rekor  map[string]*root.TransparencyLog
}

func (m *material) FulcioCertificateAuthorities() []root.CertificateAuthority {
	return []root.CertificateAuthority{m.fulcio}
}

func (m *material) RekorLogs() map[string]*root.TransparencyLog { return m.rekor }
