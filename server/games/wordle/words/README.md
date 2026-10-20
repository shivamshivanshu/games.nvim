# Wordle word lists

- `allowed.txt`: every 5-letter word of ENABLE (Enhanced North American Benchmark Lexicon),
  which is in the public domain. Source: <https://norvig.com/ngrams/enable1.txt>
  (sha256 `61ba1392a5b6199dd161fae7b483cd262fb8a88c0d49bd2c9ecb6753e755697f`), filtered with
  `LC_ALL=C grep -E '^[a-z]{5}$' enable1.txt | LC_ALL=C sort -u`.
- `answers.txt`: about 500 common words, curated by hand for this project; not taken from ENABLE.

Both are sorted (byte order), lowercase, one word per line with a trailing newline: the code
looks words up by fixed-width binary search, and `words_test.go` enforces the format.
