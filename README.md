# sid

`sid` generates **S**hort **ID**s: compact, URL-safe identifiers backed by 64 bits of cryptographic randomness. Each ID is encoded as a **13-character lowercase base32** string.

```
ab3nqykgzx4rw
mf7vt2hjcpx6q
yz9rk4bdwn2xs
```

## Features

- 64-bit cryptographic randomness via `crypto/rand`
- 13-character, lowercase base32 encoding — safe in URLs, file names, and database keys with no escaping required
- Native JSON marshal/unmarshal support
- Pack multiple IDs into a single compact string

## Installation

```sh
go get github.com/chrj/sid
```

## Usage

### Generate and parse an ID

```go
id := sid.New()
fmt.Println(id)          // ab3nqykgzx4rw

parsed := sid.Parse("ab3nqykgzx4rw")
fmt.Println(id.Equals(parsed)) // true
```

### Use as a struct field with JSON

```go
type User struct {
    ID   sid.ID `json:"id"`
    Name string `json:"name"`
}

u := User{ID: sid.New(), Name: "Alice"}

b, _ := json.Marshal(u)
// {"id":"ab3nqykgzx4rw","name":"Alice"}

var u2 User
json.Unmarshal(b, &u2)
fmt.Println(u.ID.Equals(u2.ID)) // true
```

### Pack multiple IDs into one string

```go
ids := []sid.ID{sid.New(), sid.New(), sid.New()}

packed := sid.EncodeMultiple(ids)
// "ab3nqykgzx4rwmf7vt2hjcpx6qyz9rk4bdwn2xs"

unpacked := sid.DecodeMultiple(packed)
```

This is useful for storing a list of related IDs in a single database column or URL parameter.

## Format

| Property   | Value                                    |
|------------|------------------------------------------|
| Entropy    | 64 bits (8 bytes from `crypto/rand`)     |
| Encoding   | Base32 (RFC 4648), lowercase, no padding |
| Length     | 13 characters                            |
| Alphabet   | `a–z`, `2–7`                             |

## License

MIT — see [LICENSE](LICENSE).
