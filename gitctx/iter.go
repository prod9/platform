package gitctx

import (
	gitobject "github.com/go-git/go-git/v5/plumbing/object"
	"iter"
)

func CommitSeqFromIter(iter gitobject.CommitIter) iter.Seq2[CommitRef, error] {
	return func(yield func(CommitRef, error) bool) {
		for {
			commit, err := iter.Next()
			if err != nil {
				if !yield(CommitRef{}, err) {
					return
				}
			}

			ref := CommitRef{
				Hash:    commit.Hash.String(),
				Subject: commit.Message,
			}
			if !yield(ref, nil) {
				return
			}
		}
	}
}
