# :fire: Automatic Backup

##### _Scheduled database and file backup_

<div style="text-align:center">

![GitHub commit activity](https://img.shields.io/github/commit-activity/m/mr-kaspel/automatic-site-backup)

![GitHub code size in bytes](https://img.shields.io/github/languages/code-size/mr-kaspel/automatic-site-backup)

![GitHub last commit](https://img.shields.io/github/last-commit/mr-kaspel/automatic-site-backup)

</div>

>Internal service for creating scheduled backups of site and database files

The utility will allow:

- tracking changes;
- creation of backup copies of only updated data;
- download full resource archive for the selected date;
- setting up automatic periodic tracking of data changes on the server.

### Description of work

```go
go test ./...
go build
```

```mermaid
sequenceDiagram
Client->> Server: Requesting a file
Server-->> Client: Getting the file
Client->> Database: Compare hash
Database-->> Client: Hash check result
Client->> Database: Saving a new file
```

## ✨ _Git Flow_ ✨

- git pull - update local repository
- git branch - query the current branch
- git checkout `develop` - go to branch develop
- git checkout -b `feature-detail` - create and switch to a new branch

- branch `main` - release history
- branch `develop` - all functions
- branch `feature` - individual functions

[Clue](https://www.atlassian.com/ru/git/tutorials/comparing-workflows/gitflow-workflow)