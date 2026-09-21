package authoring

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

const (
	PublicationIssueAssetUnavailable  PublicationValidationCode = "unavailable_asset_reference"
	PublicationIssueAssetIncompatible PublicationValidationCode = "incompatible_asset_media_type"
)

type PublicationAssetResolution struct {
	Bindings []courses.PublishedAssetBinding
	Issues   []PublicationValidationIssue
}

type PublicationAssetResolver interface {
	Resolve(context.Context, DraftID, ReviewSnapshot) (PublicationAssetResolution, error)
}

type PublicationAssetReader interface {
	GetAsset(context.Context, assets.AssetID) (assets.Asset, error)
}

type AssetPublicationResolver struct{ assets PublicationAssetReader }

func NewAssetPublicationResolver(repository PublicationAssetReader) *AssetPublicationResolver {
	return &AssetPublicationResolver{assets: repository}
}

type publicationAssetUse struct {
	key      string
	path     string
	required string
}

func (r *AssetPublicationResolver) Resolve(ctx context.Context, draftID DraftID, snapshot ReviewSnapshot) (PublicationAssetResolution, error) {
	if r == nil || r.assets == nil || draftID == "" || snapshot.Draft.ID != draftID {
		return PublicationAssetResolution{}, ErrPublicationInvalidInput
	}
	uses := publicationAssetUses(snapshot)
	result := PublicationAssetResolution{Bindings: make([]courses.PublishedAssetBinding, 0), Issues: make([]PublicationValidationIssue, 0)}
	resolved := make(map[string]assets.Asset)
	failed := make(map[string]struct{})
	for _, use := range uses {
		assetID, err := assets.ParseAssetID(use.key)
		if err != nil {
			result.Issues = append(result.Issues, unavailableAssetIssue(use.path))
			continue
		}
		asset, ok := resolved[use.key]
		if !ok {
			if _, alreadyFailed := failed[use.key]; alreadyFailed {
				result.Issues = append(result.Issues, unavailableAssetIssue(use.path))
				continue
			}
			asset, err = r.assets.GetAsset(ctx, assetID)
			if err != nil {
				if errors.Is(err, assets.ErrNotFound) {
					failed[use.key] = struct{}{}
					result.Issues = append(result.Issues, unavailableAssetIssue(use.path))
					continue
				}
				return PublicationAssetResolution{}, err
			}
			if asset.ID != assetID || asset.OwnerDraftID != string(draftID) || asset.Lifecycle != assets.LifecycleAvailable || asset.Validate() != nil {
				failed[use.key] = struct{}{}
				result.Issues = append(result.Issues, unavailableAssetIssue(use.path))
				continue
			}
			resolved[use.key] = asset
			result.Bindings = append(result.Bindings, courses.PublishedAssetBinding{
				AssetKey: string(asset.ID), StorageObjectID: string(asset.StorageObjectID),
				OriginalFilename: asset.OriginalFilename, MediaType: asset.MediaType,
				ByteSize: asset.ByteSize, SHA256Digest: string(asset.SHA256Digest),
			})
		}
		if use.required != "" && !strings.HasPrefix(asset.MediaType, use.required+"/") {
			result.Issues = append(result.Issues, PublicationValidationIssue{
				Code: PublicationIssueAssetIncompatible, Path: use.path,
				Message: fmt.Sprintf("The managed asset is not compatible with this %s block.", strings.ToLower(use.required)),
			})
		}
	}
	sort.Slice(result.Bindings, func(i, j int) bool { return result.Bindings[i].AssetKey < result.Bindings[j].AssetKey })
	return result, nil
}

func unavailableAssetIssue(path string) PublicationValidationIssue {
	return PublicationValidationIssue{Code: PublicationIssueAssetUnavailable, Path: path, Message: "This asset reference is not available for publication from this Draft."}
}

func validateResolvedPublication(validator PublicationValidator, cycle ReviewCycle, snapshot *ReviewSnapshot, assets PublicationAssetResolution, assessments PublicationAssessmentResolution) PublicationValidationResult {
	validation := validator.ValidateResolvedBindings(cycle, snapshot, assets.Bindings, assessments.Bindings)
	issues := make([]PublicationValidationIssue, 0, len(validation.Issues)+len(assets.Issues)+len(assessments.Issues))
	for _, issue := range validation.Issues {
		if issue.Code != PublicationIssueAssetUnresolved && issue.Code != PublicationIssueAssessmentUnresolved {
			issues = append(issues, issue)
		}
	}
	issues = append(issues, assets.Issues...)
	issues = append(issues, assessments.Issues...)
	return publicationValidationResult(issues)
}

func validatePublicationBeforeAssetResolution(validator PublicationValidator, cycle ReviewCycle, snapshot *ReviewSnapshot) PublicationValidationResult {
	validation := validator.Validate(cycle, snapshot)
	issues := make([]PublicationValidationIssue, 0, len(validation.Issues))
	for _, issue := range validation.Issues {
		if issue.Code != PublicationIssueAssetUnresolved && issue.Code != PublicationIssueAssessmentUnresolved {
			issues = append(issues, issue)
		}
	}
	return publicationValidationResult(issues)
}

func publicationAssetUses(snapshot ReviewSnapshot) []publicationAssetUse {
	uses := make([]publicationAssetUse, 0)
	for moduleIndex, module := range snapshot.Modules {
		for lessonIndex, lesson := range module.Lessons {
			for blockIndex, block := range lesson.Content.Blocks {
				base := fmt.Sprintf("modules[%d].lessons[%d].content.blocks[%d].payload", moduleIndex, lessonIndex, blockIndex)
				switch payload := block.Payload.(type) {
				case courses.ImageBlockPayload:
					uses = append(uses, publicationAssetUse{key: payload.Asset.AssetKey, path: base + ".asset.assetKey", required: "image"})
				case courses.VideoBlockPayload:
					uses = append(uses, publicationAssetUse{key: payload.Asset.AssetKey, path: base + ".asset.assetKey", required: "video"})
					uses = append(uses, publicationAssetUse{key: payload.CaptionsAsset.AssetKey, path: base + ".captionsAsset.assetKey"})
					if payload.TranscriptAsset != nil {
						uses = append(uses, publicationAssetUse{key: payload.TranscriptAsset.AssetKey, path: base + ".transcriptAsset.assetKey"})
					}
				case courses.AudioBlockPayload:
					uses = append(uses, publicationAssetUse{key: payload.Asset.AssetKey, path: base + ".asset.assetKey", required: "audio"})
					if payload.TranscriptAsset != nil {
						uses = append(uses, publicationAssetUse{key: payload.TranscriptAsset.AssetKey, path: base + ".transcriptAsset.assetKey"})
					}
				case courses.DownloadBlockPayload:
					uses = append(uses, publicationAssetUse{key: payload.Asset.AssetKey, path: base + ".asset.assetKey"})
				}
			}
		}
	}
	return uses
}
