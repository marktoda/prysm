package detection

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
	status "github.com/prysmaticlabs/prysm/slasher/db/types"
	"github.com/prysmaticlabs/prysm/slasher/detection/attestations/types"
	"go.opencensus.io/trace"
)

// DetectAttesterSlashings detects double, surround and surrounding attestation offences given an attestation.
func (ds *Service) DetectAttesterSlashings(
	ctx context.Context,
	att *ethpb.IndexedAttestation,
) ([]*ethpb.AttesterSlashing, error) {
	ctx, span := trace.StartSpan(ctx, "detection.DetectAttesterSlashings")
	defer span.End()
	results, err := ds.minMaxSpanDetector.DetectSlashingsForAttestation(ctx, att)
	if err != nil {
		return nil, err
	}
	// If the response is nil, there was no slashing detected.
	if len(results) == 0 {
		return nil, nil
	}

	var slashings []*ethpb.AttesterSlashing
	for _, result := range results {
		var slashing *ethpb.AttesterSlashing
		switch result.Kind {
		case types.DoubleVote:
			slashing, err = ds.detectDoubleVote(ctx, att, result)
			if err != nil {
				return nil, errors.Wrap(err, "could not detect double votes on attestation")
			}
		case types.SurroundVote:
			slashing, err = ds.detectSurroundVotes(ctx, att, result)
			if err != nil {
				return nil, errors.Wrap(err, "could not detect surround votes on attestation")
			}
		}
		if slashing != nil {
			slashings = append(slashings, slashing)
		}
	}

	// Clear out any duplicate results.
	keys := make(map[[32]byte]bool)
	var slashingList []*ethpb.AttesterSlashing
	for _, ss := range slashings {
		hash, err := hashutil.HashProto(ss)
		if err != nil {
			return nil, errors.Wrap(err, "could not hash slashing")
		}
		if _, value := keys[hash]; !value {
			keys[hash] = true
			slashingList = append(slashingList, ss)
		}
	}

	if err = ds.slasherDB.SaveAttesterSlashings(ctx, status.Active, slashings); err != nil {
		return nil, err
	}
	return slashingList, nil
}

// UpdateSpans passthrough function that updates span maps given an indexed attestation.
func (ds *Service) UpdateSpans(ctx context.Context, att *ethpb.IndexedAttestation) error {
	return ds.minMaxSpanDetector.UpdateSpans(ctx, att)
}

// detectDoubleVote cross references the passed in attestation with the bloom filter maintained
// for every epoch for the validator in order to determine if it is a double vote.
func (ds *Service) detectDoubleVote(
	ctx context.Context,
	incomingAtt *ethpb.IndexedAttestation,
	detectionResult *types.DetectionResult,
) (*ethpb.AttesterSlashing, error) {
	ctx, span := trace.StartSpan(ctx, "detection.detectDoubleVote")
	defer span.End()
	if detectionResult == nil || detectionResult.Kind != types.DoubleVote {
		return nil, nil
	}

	otherAtts, err := ds.slasherDB.IndexedAttestationsWithPrefix(ctx, detectionResult.SlashableEpoch, detectionResult.SigBytes[:])
	if err != nil {
		return nil, err
	}
	for _, att := range otherAtts {
		if att.Data == nil {
			continue
		}

		// If there are no shared indices, there is no validator to slash.
		if len(sliceutil.IntersectionUint64(att.AttestingIndices, []uint64{detectionResult.ValidatorIndex})) == 0 {
			continue
		}

		if isDoubleVote(incomingAtt, att) {
			doubleVotesDetected.Inc()
			return &ethpb.AttesterSlashing{
				Attestation_1: incomingAtt,
				Attestation_2: att,
			}, nil
		}

	}
	return nil, nil
}

// detectSurroundVotes cross references the passed in attestation with the requested validator's
// voting history in order to detect any possible surround votes.
func (ds *Service) detectSurroundVotes(
	ctx context.Context,
	incomingAtt *ethpb.IndexedAttestation,
	detectionResult *types.DetectionResult,
) (*ethpb.AttesterSlashing, error) {
	ctx, span := trace.StartSpan(ctx, "detection.detectSurroundVotes")
	defer span.End()
	if detectionResult == nil || detectionResult.Kind != types.SurroundVote {
		return nil, nil
	}

	otherAtts, err := ds.slasherDB.IndexedAttestationsWithPrefix(ctx, detectionResult.SlashableEpoch, detectionResult.SigBytes[:])
	if err != nil {
		return nil, err
	}
	for _, att := range otherAtts {
		if att.Data == nil {
			continue
		}
		// If there are no shared indices, there is no validator to slash.
		if len(sliceutil.IntersectionUint64(att.AttestingIndices, []uint64{detectionResult.ValidatorIndex})) == 0 {
			continue
		}

		// Slashings must be submitted as the incoming attestation surrounding the saved attestation.
		// So we swap the order if needed.
		if isSurrounding(incomingAtt, att) {
			surroundingVotesDetected.Inc()
			return &ethpb.AttesterSlashing{
				Attestation_1: incomingAtt,
				Attestation_2: att,
			}, nil
		} else if isSurrounding(att, incomingAtt) {
			surroundedVotesDetected.Inc()
			return &ethpb.AttesterSlashing{
				Attestation_1: att,
				Attestation_2: incomingAtt,
			}, nil
		}
	}
	return nil, errors.New("unexpected false positive in surround vote detection")
}

// DetectDoubleProposals checks if the given signed beacon block is a slashable offense and returns the slashing.
func (ds *Service) DetectDoubleProposals(ctx context.Context, incomingBlock *ethpb.SignedBeaconBlockHeader) (*ethpb.ProposerSlashing, error) {
	return ds.proposalsDetector.DetectDoublePropose(ctx, incomingBlock)
}

func isDoubleVote(incomingAtt *ethpb.IndexedAttestation, prevAtt *ethpb.IndexedAttestation) bool {
	return !proto.Equal(incomingAtt.Data, prevAtt.Data) && incomingAtt.Data.Target.Epoch == prevAtt.Data.Target.Epoch
}

func isSurrounding(incomingAtt *ethpb.IndexedAttestation, prevAtt *ethpb.IndexedAttestation) bool {
	return incomingAtt.Data.Source.Epoch < prevAtt.Data.Source.Epoch &&
		incomingAtt.Data.Target.Epoch > prevAtt.Data.Target.Epoch
}
