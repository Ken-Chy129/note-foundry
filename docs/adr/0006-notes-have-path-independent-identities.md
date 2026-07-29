# Notes have path-independent identities

Every Learning Note has an immutable identifier independent of its title, slug, Knowledge Space, and directory position. Internal links and public URLs resolve through that identifier so reorganizing or renaming knowledge does not break references; readable slugs remain mutable presentation data, and Markdown export translates internal identifiers into portable relative links.

Canonical Markdown encodes an internal link as a standard Markdown link whose destination is `note:<uuid>`, for example `[Memory Manager](note:11111111-1111-4111-8111-111111111111)`. The runtime derives forward links and backlinks from these destinations, while export rewrites them to portable relative paths.
