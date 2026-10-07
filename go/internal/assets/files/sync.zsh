#!/usr/bin/env zsh
emulate -LR zsh
setopt NO_ALIASES ERR_EXIT NO_UNSET PIPE_FAIL
trap 'print -ru2 -- "sync failed at native line $LINENO; local and remote effects are retained"' ZERR

[[ $1 == sync ]]
skill_root=$2
private_root=$3
patch_tag=$4
public_root=$5
repository=$6
version=$7
default_branch=$8
attempt_root=$9
remote_url=https://github.com/$repository.git

for source_root in "$skill_root" "$private_root"; do
    [[ $(/usr/bin/git -C "$source_root" rev-parse --show-toplevel) == "$source_root" ]]
    [[ $(/usr/bin/git -C "$source_root" symbolic-ref --short HEAD) == main ]]
done
skill_status=$(/usr/bin/git -C "$skill_root" status --porcelain=v1 --untracked-files=all)
[[ -z $skill_status ]]
[[ -d $private_root/$patch_tag && ! -L $private_root/$patch_tag ]]
[[ -n $(/usr/bin/git -C "$private_root" ls-files -- "$patch_tag/") ]]
patch_status=$(/usr/bin/git -C "$private_root" status --porcelain=v1 --untracked-files=all --ignored -- "$patch_tag/")
[[ -z $patch_status ]]
/usr/bin/git -C "$skill_root" archive --format=tar --output="$attempt_root/skill.tar" HEAD
/usr/bin/git -C "$private_root" archive --format=tar --output="$attempt_root/patches.tar" HEAD "$patch_tag"

if [[ ! -e $public_root ]]; then
    /usr/bin/gh repo clone "$remote_url" "$public_root" --no-upstream
fi
[[ $(/usr/bin/git -C "$public_root" rev-parse --show-toplevel) == "$public_root" ]]
[[ $(/usr/bin/git -C "$public_root" remote get-url origin) == "$remote_url" ]]
public_status=$(/usr/bin/git -C "$public_root" status --porcelain=v1 --untracked-files=all --ignored)
[[ -z $public_status ]]
if [[ -z $default_branch ]]; then
    ! /usr/bin/git -C "$public_root" rev-parse --verify HEAD >/dev/null 2>&1
    /usr/bin/git -C "$public_root" symbolic-ref HEAD refs/heads/main
else
    [[ $(/usr/bin/git -C "$public_root" symbolic-ref --short HEAD) == main ]]
    /usr/bin/git -C "$public_root" fetch --quiet origin main
    /usr/bin/git -C "$public_root" -c core.hooksPath=/dev/null merge --quiet --ff-only origin/main
fi

/usr/bin/git -C "$public_root" rm --quiet -r --ignore-unmatch -- .
/usr/bin/tar -xf "$attempt_root/skill.tar" -C "$public_root"
/usr/bin/mkdir -p -- "$public_root/patches"
/usr/bin/tar -xf "$attempt_root/patches.tar" -C "$public_root/patches"
/usr/bin/git -C "$public_root" add -f --all -- .
export GIT_AUTHOR_NAME=nobody GIT_AUTHOR_EMAIL=nobody@nobody
export GIT_COMMITTER_NAME=nobody GIT_COMMITTER_EMAIL=nobody@nobody
/usr/bin/git -C "$public_root" -c core.hooksPath=/dev/null commit --quiet --no-gpg-sign --allow-empty -m "$version"
[[ $(/usr/bin/git -C "$public_root" log -1 --format=%B) == "$version" ]]
[[ $(/usr/bin/git -C "$public_root" log -1 --format='%an <%ae>') == 'nobody <nobody@nobody>' ]]
[[ $(/usr/bin/git -C "$public_root" log -1 --format='%cn <%ce>') == 'nobody <nobody@nobody>' ]]
/usr/bin/git -C "$public_root" -c core.hooksPath=/dev/null -c credential.helper= -c 'credential.helper=!/usr/bin/gh auth git-credential' push --quiet origin main
/usr/bin/git -C "$public_root" fetch --quiet origin main
/usr/bin/git -C "$public_root" diff --quiet --no-ext-diff --no-textconv HEAD origin/main -- .
[[ $(/usr/bin/gh api "repos/$repository/commits/main" --jq .commit.message) == "$version" ]]
