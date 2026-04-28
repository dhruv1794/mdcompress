# edge cases

Trailing whitespace at the end of this line should survive a roundtrip.   

Tabs	in	prose	too.

	indented code block
	with two lines

```
fenced
block
with no language
```

```text
fenced with a language
```

> nested
> > blockquote
> > continued

| col a | col b |
| --- | --- |
| 1 | 2 |
| 3 | 4 |

[ref-style link][ref]

[ref]: https://example.com "title"

<details>
<summary>collapsed by default</summary>

inner content

</details>

End of file with a single trailing newline.
