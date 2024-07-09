package gitmetrics

import (
	"time"
)

type Commit struct {
	CommitMessage string    `bson:"commit_message"`
	LinesDeleted  int       `bson:"lines_deleted"`
	CommitID      string    `bson:"commit_id"`
	CommittedBy   string    `bson:"commited_by"`
	LinesAdded    int       `bson:"lines_added"`
	RepoName      string    `bson:"reponame"`
	CommitDate    time.Time `bson:"commit_date"`
	FilesAdded    int       `bson:"files_added"`
	FilesDeleted  int       `bson:"files_deleted"`
	FilesUpdated  int       `bson:"files_updated"`
}

type Repository struct {
	Name string `json:"name"`
}
