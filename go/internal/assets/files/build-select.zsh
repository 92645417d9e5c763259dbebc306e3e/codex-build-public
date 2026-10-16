#!/usr/bin/env zsh
emulate -LR zsh
setopt NO_ALIASES ERR_EXIT NO_UNSET PIPE_FAIL

dist_root=$2
build_version=$3
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
