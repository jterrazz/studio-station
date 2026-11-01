# The footprint test's matcher. The first file holds the allowed paths, one shell
# glob per line relative to $HOME (the storage table's paths, the home-root
# exceptions, the OS's noise); the second the paths a step created, relative to
# $HOME. A path is matched when it is an allowed path, lies under one, or is a
# parent folder of one (~/.local made for ~/.local/share/mise). Prints every
# unmatched path.
# usage: awk -f footprint.awk allowed.txt created.txt
function regex(glob,    out, i, c) {
    out = ""
    for (i = 1; i <= length(glob); i++) {
        c = substr(glob, i, 1)
        if (c == "*") out = out "[^/]*"
        else if (c == "?") out = out "[^/]"
        else if (index("\\.^$+()[]{}|", c)) out = out "\\" c
        else out = out c
    }
    return out
}
function literal(glob,    n) {
    n = match(glob, /[*?]/)
    return n ? substr(glob, 1, n - 1) : glob
}
FNR == NR {
    if ($0 == "" || $0 ~ /^#/) next
    allowed[++count] = $0
    re[count] = "^" regex($0) "(/.*)?$"
    lit[count] = literal($0)
    next
}
{
    path = $0
    for (i = 1; i <= count; i++) {
        if (path ~ re[i]) next
        if (index(lit[i], path "/") == 1) next
    }
    print path
}
