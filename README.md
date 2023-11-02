# module-migration cli

Search and replace ustility to migrate Go modules from one location to another location.

## Requirements:
 - git client with access to the source and target repos
 - gh client for creating github pull requests
 - go toolchain for dependency management and build checks

## Installation

```shell
go install github.com/jxsl13/module-migration@latest
```


## Usage

mapping.csv
```csv
Repo-clone-url;Target-Clone-Url
ssh://git@git.company.com/project/repo.git;git@github.com:company/new-repo.git
```

```shell
export MM_CSV="/home/user/Desktop/module-migration/replace.csv"
# also possible to use index based column definitions
export MM_OLD="Repo-clone-url"
export MM_NEW=" Target-Clone-Url"
# Excel exports with ; as separator ba default, that's why that is the default
export MM_SEPARATOR=";"
# folder to copy
export MM_COPY="/home/user/GitHubMigration/.github"

module-migration migrate ./
module-migration commit ./
module-migration release ./ --push
```

## module-migration migrate
```shell
$ module-migration migrate --help

  MM_CSV          path to csv mapping file (default: "./mapping.csv")
  MM_SEPARATOR    column separator character in csv (default: ";")
  MM_OLD          column name or index (starting with 0) containing the old [git] url (default: "0")
  MM_NEW          column name or index (starting with 0) containing the new [git] url (default: "1")
  MM_REMOTE       name of the remote url (default: "origin")
  MM_BRANCH       name of the branch that should be crated for the changes, if empty no branch migration will be executed with git (default: "chore/module-migration")
  MM_INCLUDE      ',' separated list of include file paths matching regular expression (default: "\\.go$,Dockerfile$,Jenkinsfile$,\\.yaml$,\\.yml$,\\.md$,\\.MD$")
  MM_EXCLUDE      ',' separated list of exclude file paths matching regular expression (default: "\\.git$")
  MM_COPY         moves specified files or directories into your repository (, separated)

Usage:
  module-migration migrate [flags]

Flags:
  -b, --branch string      name of the branch that should be crated for the changes, if empty no branch migration will be executed with git (default "chore/module-migration")
      --copy string        moves specified files or directories into your repository (, separated)
  -c, --csv string         path to csv mapping file (default "./mapping.csv")
  -e, --exclude string     ',' separated list of exclude file paths matching regular expression (default "\\.git$")
  -h, --help               help for migrate
  -i, --include string     ',' separated list of include file paths matching regular expression (default "\\.go$,Dockerfile$,Jenkinsfile$,\\.yaml$,\\.yml$,\\.md$,\\.MD$")
  -n, --new string         column name or index (starting with 0) containing the new [git] url (default "1")
  -o, --old string         column name or index (starting with 0) containing the old [git] url (default "0")
  -r, --remote string      name of the remote url (default "origin")
  -s, --separator string   column separator character in csv (default ";")
```

## module-migration commit
```shell
$ module-migration commit --help

  MM_CSV          path to csv mapping file (default: "./mapping.csv")
  MM_SEPARATOR    column separator character in csv (default: ";")
  MM_OLD          column name or index (starting with 0) containing the old [git] url (default: "0")
  MM_NEW          column name or index (starting with 0) containing the new [git] url (default: "1")
  MM_REMOTE       name of the remote url (default: "origin")
  MM_BRANCH       name of the branch that should be crated for the changes, if empty no branch migration will be executed with git (default: "chore/module-migration")

Usage:
  module-migration commit [flags]

Flags:
  -b, --branch string      name of the branch that should be crated for the changes, if empty no branch migration will be executed with git (default "chore/module-migration")
  -c, --csv string         path to csv mapping file (default "./mapping.csv")
  -h, --help               help for commit
  -n, --new string         column name or index (starting with 0) containing the new [git] url (default "1")
  -o, --old string         column name or index (starting with 0) containing the old [git] url (default "0")
  -r, --remote string      name of the remote url (default "origin")
  -s, --separator string   column separator character in csv (default ";")
```

## module-migration release
```shell
module-migration release --help

  MM_REMOTE    name of the remote url (default: "origin")
  MM_PUSH      push tags to remote repo (default: "false")

Usage:
  module-migration release [flags]

Flags:
  -h, --help            help for release
  -p, --push            push tags to remote repo
  -r, --remote string   name of the remote url (default "origin")
```