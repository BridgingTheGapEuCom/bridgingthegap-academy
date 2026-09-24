// Package plugins owns plugin package identity, validation, trust, and the local
// installation registry. It never executes plugin resources.
package plugins

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

const (
	PackageFormat        = "bridging-the-gap-plugin"
	PackageFormatVersion = 1
	manifestPath         = "manifest.json"
	checksumsPath        = "checksums.json"
	signaturePath        = "signature.json"
)

type PluginID string
type PluginType string
type Permission string

const (
	TypeCourseWidget    PluginType = "COURSE_WIDGET"
	TypeDashboardWidget PluginType = "DASHBOARD_WIDGET"
	PermissionNone      Permission = "NONE"
)

var (
	pluginIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9]+(?:[a-z0-9-]*[a-z0-9])?){2,}$`)
	widgetIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	keyIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/#-]{0,239}$`)
)

type Publisher struct {
	Name string `json:"name"`
}

type Entrypoint struct {
	ID       string     `json:"id"`
	Type     PluginType `json:"type"`
	Name     string     `json:"name"`
	Resource string     `json:"resource"`
}

type Resource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Manifest struct {
	Format        string       `json:"format"`
	FormatVersion int          `json:"formatVersion"`
	ID            PluginID     `json:"id"`
	Version       string       `json:"version"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	Publisher     Publisher    `json:"publisher"`
	Homepage      string       `json:"homepage,omitempty"`
	PluginTypes   []PluginType `json:"pluginTypes"`
	Entrypoints   []Entrypoint `json:"entrypoints"`
	Permissions   []Permission `json:"permissions"`
	Resources     []Resource   `json:"resources"`
}

type Checksums struct {
	Algorithm string     `json:"algorithm"`
	Entries   []Resource `json:"entries"`
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId"`
	Value     string `json:"value"`
}

type ErrorCode string

const (
	ErrInvalidArchive     ErrorCode = "invalid_archive"
	ErrPackageTooLarge    ErrorCode = "package_too_large"
	ErrInvalidManifest    ErrorCode = "invalid_manifest"
	ErrUnsupportedFormat  ErrorCode = "unsupported_package_format"
	ErrUnsupportedVersion ErrorCode = "unsupported_package_version"
	ErrChecksumMismatch   ErrorCode = "checksum_mismatch"
	ErrInvalidSignature   ErrorCode = "invalid_signature"
)

type PackageError struct {
	Code     ErrorCode
	Location string
}

func (e *PackageError) Error() string {
	if e.Location == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Location
}
func packageError(c ErrorCode, l string) error { return &PackageError{Code: c, Location: l} }
func IsPackageError(err error, code ErrorCode) bool {
	var target *PackageError
	return errors.As(err, &target) && target.Code == code
}

type Limits struct {
	MaxCompressedBytes, MaxUncompressedBytes, MaxEntryBytes int64
	MaxEntries                                              int
	MaxCompressionRatio                                     int64
}

func DefaultLimits() Limits { return Limits{64 << 20, 256 << 20, 64 << 20, 1024, 200} }
func (l Limits) normalized() Limits {
	d := DefaultLimits()
	if l.MaxCompressedBytes <= 0 {
		l.MaxCompressedBytes = d.MaxCompressedBytes
	}
	if l.MaxUncompressedBytes <= 0 {
		l.MaxUncompressedBytes = d.MaxUncompressedBytes
	}
	if l.MaxEntryBytes <= 0 {
		l.MaxEntryBytes = d.MaxEntryBytes
	}
	if l.MaxEntries <= 0 {
		l.MaxEntries = d.MaxEntries
	}
	if l.MaxCompressionRatio <= 0 {
		l.MaxCompressionRatio = d.MaxCompressionRatio
	}
	return l
}

type PackageReader struct{ limits Limits }

func NewPackageReader(l Limits) *PackageReader { return &PackageReader{limits: l.normalized()} }

// ValidatedPluginPackage cannot be constructed with a useful value outside this
// package. It contains data only; validation never loads or executes resources.
type ValidatedPluginPackage struct {
	manifest       Manifest
	resources      map[string][]byte
	artifactDigest string
	signingPayload []byte
	signature      *Signature
}

func (p *ValidatedPluginPackage) Manifest() Manifest {
	if p == nil {
		return Manifest{}
	}
	return cloneManifest(p.manifest)
}
func (p *ValidatedPluginPackage) ArtifactDigest() string {
	if p == nil {
		return ""
	}
	return p.artifactDigest
}
func (p *ValidatedPluginPackage) Signature() (Signature, bool) {
	if p == nil || p.signature == nil {
		return Signature{}, false
	}
	return *p.signature, true
}
func (p *ValidatedPluginPackage) SigningPayload() []byte {
	if p == nil {
		return nil
	}
	return append([]byte(nil), p.signingPayload...)
}
func (p *ValidatedPluginPackage) Resource(name string) (io.ReadCloser, error) {
	b, ok := p.resources[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (r *PackageReader) Read(ctx context.Context, input io.Reader) (*ValidatedPluginPackage, error) {
	if r == nil || input == nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	l := r.limits.normalized()
	f, err := os.CreateTemp("", "btg-plugin-*.zip")
	if err != nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	n, err := io.Copy(f, io.LimitReader(input, l.MaxCompressedBytes+1))
	if err == nil && n > l.MaxCompressedBytes {
		err = errors.New("too large")
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return nil, packageError(ErrPackageTooLarge, "")
	}
	f, err = os.Open(name)
	if err != nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	defer func() { _ = f.Close() }()
	zr, err := zip.NewReader(f, n)
	if err != nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	return r.validate(ctx, zr)
}

func (r *PackageReader) validate(ctx context.Context, z *zip.Reader) (*ValidatedPluginPackage, error) {
	l := r.limits.normalized()
	if len(z.File) > l.MaxEntries {
		return nil, packageError(ErrPackageTooLarge, "entries")
	}
	raw := map[string][]byte{}
	var total int64
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name, err := safePath(f.Name)
		if err != nil {
			return nil, err
		}
		if _, ok := raw[name]; ok {
			return nil, packageError(ErrInvalidArchive, name)
		}
		if f.FileInfo().Mode()&os.ModeType != 0 {
			return nil, packageError(ErrInvalidArchive, name)
		}
		if f.UncompressedSize64 > uint64(l.MaxEntryBytes) || (f.CompressedSize64 > 0 && f.UncompressedSize64 > f.CompressedSize64*uint64(l.MaxCompressionRatio)) {
			return nil, packageError(ErrPackageTooLarge, name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, packageError(ErrInvalidArchive, name)
		}
		b, readErr := io.ReadAll(io.LimitReader(rc, l.MaxEntryBytes+1))
		closeErr := rc.Close()
		if readErr != nil || closeErr != nil {
			return nil, packageError(ErrInvalidArchive, name)
		}
		if int64(len(b)) > l.MaxEntryBytes {
			return nil, packageError(ErrPackageTooLarge, name)
		}
		total += int64(len(b))
		if total > l.MaxUncompressedBytes {
			return nil, packageError(ErrPackageTooLarge, "total")
		}
		raw[name] = b
	}
	if raw[manifestPath] == nil || raw[checksumsPath] == nil {
		return nil, packageError(ErrInvalidArchive, "required entry")
	}
	var m Manifest
	if err := strictDecode(raw[manifestPath], &m); err != nil {
		return nil, packageError(ErrInvalidManifest, manifestPath)
	}
	if m.Format != PackageFormat {
		return nil, packageError(ErrUnsupportedFormat, manifestPath)
	}
	if m.FormatVersion != PackageFormatVersion {
		return nil, packageError(ErrUnsupportedVersion, manifestPath)
	}
	if err := m.Validate(); err != nil {
		return nil, packageError(ErrInvalidManifest, err.Error())
	}
	var checks Checksums
	if err := strictDecode(raw[checksumsPath], &checks); err != nil || checks.Algorithm != "SHA-256" {
		return nil, packageError(ErrInvalidManifest, checksumsPath)
	}
	if !sameResources(m.Resources, checks.Entries) {
		return nil, packageError(ErrChecksumMismatch, checksumsPath)
	}
	declared := map[string]bool{manifestPath: true, checksumsPath: true}
	if raw[signaturePath] != nil {
		declared[signaturePath] = true
	}
	resources := map[string][]byte{}
	for _, entry := range m.Resources {
		if !strings.HasPrefix(entry.Path, "resources/") {
			return nil, packageError(ErrInvalidManifest, entry.Path)
		}
		b, ok := raw[entry.Path]
		if !ok || int64(len(b)) != entry.Size || digest(b) != entry.SHA256 {
			return nil, packageError(ErrChecksumMismatch, entry.Path)
		}
		declared[entry.Path] = true
		resources[entry.Path] = append([]byte(nil), b...)
	}
	for name := range raw {
		if !declared[name] {
			return nil, packageError(ErrInvalidArchive, name)
		}
	}
	canonical, err := canonicalManifest(m)
	if err != nil {
		return nil, packageError(ErrInvalidManifest, manifestPath)
	}
	artifact := artifactDigest(canonical, m.Resources)
	payload := canonicalSigningPayload(canonical, artifact)
	var sig *Signature
	if b := raw[signaturePath]; b != nil {
		var value Signature
		if err := strictDecode(b, &value); err != nil || value.Algorithm != "Ed25519" || !keyIDPattern.MatchString(value.KeyID) {
			return nil, packageError(ErrInvalidSignature, signaturePath)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(value.Value)
		if err != nil || len(decoded) != ed25519.SignatureSize {
			return nil, packageError(ErrInvalidSignature, signaturePath)
		}
		sig = &value
	}
	return &ValidatedPluginPackage{manifest: cloneManifest(m), resources: resources, artifactDigest: artifact, signingPayload: payload, signature: sig}, nil
}

func (m Manifest) Validate() error {
	if m.Format != PackageFormat {
		return errors.New("format")
	}
	if m.FormatVersion != PackageFormatVersion {
		return errors.New("formatVersion")
	}
	if !pluginIDPattern.MatchString(string(m.ID)) || len(m.ID) > 240 {
		return errors.New("id")
	}
	if _, err := courses.ParseVersion(m.Version); err != nil {
		return errors.New("version")
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 160 || len(m.Description) > 2000 || strings.TrimSpace(m.Publisher.Name) == "" || len(m.Publisher.Name) > 160 {
		return errors.New("metadata")
	}
	if m.Homepage != "" {
		u, err := url.Parse(m.Homepage)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
			return errors.New("homepage")
		}
	}
	if len(m.PluginTypes) < 1 || len(m.PluginTypes) > 2 || len(m.Entrypoints) < 1 {
		return errors.New("pluginTypes")
	}
	types := map[PluginType]bool{}
	for _, t := range m.PluginTypes {
		if t != TypeCourseWidget && t != TypeDashboardWidget {
			return errors.New("pluginTypes")
		}
		if types[t] {
			return errors.New("pluginTypes")
		}
		types[t] = true
	}
	permissions := map[Permission]bool{}
	for _, p := range m.Permissions {
		if p != PermissionNone || permissions[p] {
			return errors.New("permissions")
		}
		permissions[p] = true
	}
	if len(m.Permissions) != 1 || m.Permissions[0] != PermissionNone {
		return errors.New("permissions")
	}
	widgets := map[string]bool{}
	resources := map[string]Resource{}
	last := ""
	for _, v := range m.Resources {
		if v.Path <= last || v.Size < 0 || !validDigest(v.SHA256) {
			return errors.New("resources")
		}
		if _, err := safePath(v.Path); err != nil {
			return errors.New("resources")
		}
		resources[v.Path] = v
		last = v.Path
	}
	for _, e := range m.Entrypoints {
		if !widgetIDPattern.MatchString(e.ID) || widgets[e.ID] || !types[e.Type] || strings.TrimSpace(e.Name) == "" || resources[e.Resource].Path == "" {
			return errors.New("entrypoints")
		}
		widgets[e.ID] = true
	}
	return nil
}

func safePath(v string) (string, error) {
	if v == "" || strings.Contains(v, "\\") || strings.HasPrefix(v, "/") || path.Clean(v) != v || v == "." || strings.HasPrefix(v, "../") || strings.ContainsRune(v, '\x00') {
		return "", packageError(ErrInvalidArchive, "path")
	}
	return v, nil
}
func strictDecode(b []byte, out any) error {
	if !json.Valid(b) || duplicateJSONKey(b) {
		return errors.New("invalid json")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing json")
	}
	return nil
}
func duplicateJSONKey(b []byte) bool {
	d := json.NewDecoder(bytes.NewReader(b))
	var walk func() bool
	walk = func() bool {
		t, e := d.Token()
		if e != nil {
			return true
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return true
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return true
				}
				seen[s] = true
				if walk() {
					return true
				}
			}
			_, e = d.Token()
			return e != nil
		case json.Delim('['):
			for d.More() {
				if walk() {
					return true
				}
			}
			_, e = d.Token()
			return e != nil
		}
		return false
	}
	return walk()
}
func validDigest(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil && v == strings.ToLower(v)
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func sameResources(a, b []Resource) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func canonicalManifest(m Manifest) ([]byte, error) { return json.Marshal(m) }
func artifactDigest(manifest []byte, entries []Resource) string {
	h := sha256.New()
	h.Write([]byte("btg-plugin-artifact-v1\n"))
	h.Write(manifest)
	for _, e := range entries {
		_, _ = h.Write([]byte("\n" + e.Path + "\x00" + e.SHA256 + "\x00" + strconv.FormatInt(e.Size, 10)))
	}
	return hex.EncodeToString(h.Sum(nil))
}
func canonicalSigningPayload(manifest []byte, artifact string) []byte {
	return []byte("btg-plugin-signature-v1\n" + artifact + "\n" + string(manifest))
}
func cloneManifest(m Manifest) Manifest {
	m.PluginTypes = append([]PluginType(nil), m.PluginTypes...)
	m.Entrypoints = append([]Entrypoint(nil), m.Entrypoints...)
	m.Permissions = append([]Permission(nil), m.Permissions...)
	m.Resources = append([]Resource(nil), m.Resources...)
	return m
}

// PackageFixture is deliberately limited to tests/tooling that need to produce
// the canonical v1 representation; runtime registration still requires Reader.
func BuildPackageForTesting(m Manifest, contents map[string][]byte, keyID string, private ed25519.PrivateKey) ([]byte, error) {
	paths := make([]string, 0, len(contents))
	for p := range contents {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	m.Resources = nil
	for _, p := range paths {
		m.Resources = append(m.Resources, Resource{Path: p, SHA256: digest(contents[p]), Size: int64(len(contents[p]))})
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	manifest, _ := json.Marshal(m)
	checks, _ := json.Marshal(Checksums{Algorithm: "SHA-256", Entries: m.Resources})
	artifact := artifactDigest(manifest, m.Resources)
	var sig []byte
	if len(private) > 0 {
		value := Signature{Algorithm: "Ed25519", KeyID: keyID, Value: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, canonicalSigningPayload(manifest, artifact)))}
		sig, _ = json.Marshal(value)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	entries := map[string][]byte{manifestPath: manifest, checksumsPath: checks}
	if sig != nil {
		entries[signaturePath] = sig
	}
	for p, b := range contents {
		entries[p] = b
	}
	names := make([]string, 0, len(entries))
	for p := range entries {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		w, e := zw.Create(p)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(entries[p]); e != nil {
			return nil, e
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
