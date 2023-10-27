# module-migration cli

Search and replace ustility to migrate Go modules from one location to another location.

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
module-migration ./ --csv mapping.csv --separator ';' --old 'Repo-clone-url' --new 'Target-Clone-Url'

# or by column index

module-migration ./ --csv mapping.csv --separator ';' --old 0 --new 1


module-migration /home/user/sourceDir --csv /home/behm015/Desktop/module-migration/replace.csv --old Repo-clone-url --new Target-Clone-Url --separator ";" --branch chore/migrate-imports --copy /home/user/GitHubMigration/.github
```
