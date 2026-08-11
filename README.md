# sid

`sid` generates **S**hort **ID**s: compact, URL-safe identifiers backed by cryptographic randomness. An ID holds 40, 80, 120, or 160 bits. It encodes as a lowercase base32 string of 8, 16, 24, or 32 characters.

```
ab3nqykg                          40-bit
mf7vt2hjcpx6qw4d                  80-bit
drn7oow74uhzhjjwl5xasnp6          120-bit
6hhehlttai7vkdg25p6buouh5mju2zbz  160-bit
```

## Features

- Four sizes, from 40 to 160 bits, from `crypto/rand`
- Lowercase base32 output. It is safe in URLs, file names, and database keys with no escape characters
- One ID has one string form, and one string has one meaning
- `ID` is comparable. You can use `==` on it, and you can use it as a map key
- Text and JSON support through `encoding.TextMarshaler`
- No panics on input from a caller. Every failure returns an error
- Pack many IDs of one size into a single string

## Installation

```sh
go get github.com/chrj/sid
```

## Usage

### Generate an ID

Pass the size you want to `New`:

```go
id, err := sid.New(sid.Size80)
if err != nil {
    return err
}

fmt.Println(id) // mf7vt2hjcpx6qw4d
```

`New` returns an error only when the size is not one of the four constants. For a size that is fixed in code, use `MustNew`:

```go
var nodeID = sid.MustNew(sid.Size80)
```

### Parse an ID

`Parse` reads the size from the length of the string:

```go
id, err := sid.Parse("mf7vt2hjcpx6qw4d")
if err != nil {
    return err
}

fmt.Println(id.Size()) // 80-bit
```

`Parse` accepts input from a caller, so it always returns an error instead of a panic. Input must be lowercase. The empty string parses to the zero ID.

### Compare IDs

`ID` is comparable, so `==` and map keys work:

```go
if a == b {
    // the same ID
}

seen := map[sid.ID]bool{}
seen[id] = true
```

An ID of one size never equals an ID of another size.

### Use as a struct field with JSON

```go
type User struct {
    ID   sid.ID `json:"id"`
    Name string `json:"name"`
}

u := User{ID: sid.MustNew(sid.Size80), Name: "Alice"}

b, err := json.Marshal(u)
// {"id":"mf7vt2hjcpx6qw4d","name":"Alice"}
```

A JSON `null`, a missing field, and an empty string all decode to the zero ID. Any other invalid value returns an error.

### Pack many IDs into one string

All IDs must have the same size, because the decoder splits on a fixed width:

```go
ids := []sid.ID{sid.MustNew(sid.Size80), sid.MustNew(sid.Size80)}

packed, err := sid.EncodeMultiple(ids)
// "mf7vt2hjcpx6qw4d2uatqdrm27w4abvu"

unpacked, err := sid.DecodeMultiple(packed, sid.Size80)
```

This is useful to store a list of related IDs in one database column or URL parameter.

## Format

| Size constant | Bits | Bytes | Characters | 50% collision after |
|---------------|------|-------|------------|---------------------|
| `Size40`      | 40   | 5     | 8          | 1.2e6 IDs           |
| `Size80`      | 80   | 10    | 16         | 1.3e12 IDs          |
| `Size120`     | 120  | 15    | 24         | 1.4e18 IDs          |
| `Size160`     | 160  | 20    | 32         | 1.4e24 IDs          |

Encoding is base32 (RFC 4648) with the lowercase alphabet `a-z` and `2-7`, and no padding.

Every size is a multiple of 5 bytes, which is the block size of base32. Two results come from this property. The encoded length is always a whole number of characters, and the encoding is a bijection. Each ID has exactly one string form, and each string of a supported length decodes to exactly one ID.

A size that is not a multiple of 5 bytes leaves spare bits in the last character. More than one string then decodes to the same ID, and the last character can hold only some of the 32 symbols.

## Collisions

IDs are random, so two of them can collide. The risk stays near zero until the count approaches the square root of the value space. The table above gives the count at which the risk reaches 50 percent.

Use `Size40` only where a collision is cheap to detect, such as a short code behind a unique index. For a primary key, use `Size80` or larger.

## Errors

| Type             | Cause                                                        |
|------------------|--------------------------------------------------------------|
| `SizeError`      | The size is not one of the four constants                     |
| `ParseError`     | The string has the wrong length or holds an invalid character |
| `MixedSizeError` | `EncodeMultiple` received IDs of more than one size            |

Use `errors.As` to read the fields. `ParseError` carries the input and a `Reason` that names the rule that the input broke.

Only `MustNew` and `MustParse` panic. Use them for values that are fixed in code, never for input from a caller.

## License

MIT — see [LICENSE](LICENSE).
