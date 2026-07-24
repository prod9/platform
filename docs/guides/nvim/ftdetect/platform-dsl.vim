" Vim filetype detection
" Language: platform manifest-patch DSL (.platform files)

au BufRead,BufNewFile *.platform setfiletype platform-dsl
