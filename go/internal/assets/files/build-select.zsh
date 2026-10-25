#!/usr/bin/env zsh
emulate -LR zsh
setopt NO_ALIASES ERR_EXIT NO_UNSET PIPE_FAIL

dist_root=$2
build_version=$3
if [[ $1 == prepare ]]; then
    private_root=${dist_root:h}/patches/private
    [[ ! -e $private_root && ! -L $private_root ]] && exit 0
    [[ -d $private_root && ! -L $private_root && ${private_root:A} == "$private_root" ]]
    previous_root=$private_root/$4-prev
    [[ ! -e $previous_root && ! -L $previous_root ]] && exit 0
    [[ -d $previous_root && ! -L $previous_root ]]
    [[ $(/usr/bin/git -C "$private_root" rev-parse --show-toplevel) == "$private_root" ]]
    [[ -z $(/usr/bin/git -C "$private_root" ls-files -- "$4-prev/") ]]
    /usr/bin/rm -r -- "$previous_root"
    /usr/bin/sync -- "$private_root"
    exit 0
fi
current_link=$dist_root/current
if [[ $1 == replace-draft ]]; then
    [[ -L $current_link && $(/usr/bin/readlink -- "$current_link") == "$build_version" ]]
    /usr/bin/mv --exchange --no-copy -T -- "$dist_root/.$build_version.stage" "$dist_root/$build_version"
else
    stage_link=$dist_root/.current-stage
    [[ ! -e $current_link || -L $current_link ]]
    if [[ ! -L $current_link || $(/usr/bin/readlink -- "$current_link") != "$build_version" ]]; then
        if [[ -L $stage_link ]]; then
            [[ $(/usr/bin/readlink -- "$stage_link") == "$build_version" ]]
        else
            /usr/bin/ln -s -T -- "$build_version" "$stage_link"
        fi
        trap '/usr/bin/rm -f -- "$stage_link"' EXIT
        /usr/bin/mv -T -- "$stage_link" "$current_link"
    fi
fi
/usr/bin/sync -- "$dist_root/$build_version" "$dist_root"
