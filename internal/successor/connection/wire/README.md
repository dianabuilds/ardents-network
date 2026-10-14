# Service Connection wire

Interface: `Read`, `Write`, `ReadStream` implement the closed v3 record grammar.
`InitialRequestBytes` and `InitialResponseBytes` bound exactly the two fixed
records in each initial coalesced flight. They permit bounded read-ahead without
consuming any following Application record and change no accepted encoding.
`Read` coalesces only the fixed prefix/length header in a bounded buffer; it
checks a complete bad prefix without awaiting more bytes, bounds the body before
allocation and leaves the body and next record with the original reader. A
failed header read cannot become successful through buffering returned bytes.
`ErrInvalidRecord` identifies an observed contradiction of that grammar,
including an invalid complete prefix or length before body acquisition.
Physical read failures and incomplete frames preserve their original I/O cause
without that marker. A complete authentication record on the active-stream
reader is a grammar-phase contradiction. None of these classifications proves
why transport failed or authorizes recovery.
`Contexts` binds a copied immutable logical tuple and the separate fresh
Attachment. `NewContinuity`/`VerifyContinuity` own its fixed MAC/nonce encodings.
`BindTLS` seals the actual completed TLS1.3 object and fresh exporter.
`VerifyInitialRequest` validates zero-offset coalesced Continuity before issuing
an exact-TLS, single-use challenge receipt; Instance separately verifies its
expected tuple before key use.
`NewInitialResponse` verifies the exact issued Instance proof and zero-offset
Publisher Continuity and seals one coalesced response for that same TLS object.
Copied, foreign and consumed receipts refuse. `Erase` retires exporter material
after the owning TLS users join; it cannot revoke previously copied secret bytes.

`VerifyReplacement` authenticates the fresh exporter with the original logical
continuity key, then seals one exact-TLS receipt. `NewReplacementResponse`
consumes that checked request to seal one opposite-role Continuity flight.
The fresh TLS-derived key cannot replace the original secret. These receipts
do not validate retained byte ranges, admit a proposal, join old I/O or commit
a logical generation; those responsibilities remain with Connection.

Implementation separates record allocation/encoding, identity transcript,
exporter derivation, Continuity MAC and initial receipt ownership. The Module
has no authority, resource acquisition or domain callback. Connection's actual
recipient binding uses the same canonical context construction as Instance.
Independent canonical byte/HMAC vectors, closed-grammar refusals and receipt
ownership tests cover mechanisms. Real TLS fixtures live with the Connection
integration tests; none establishes Publication/Execution/Route readiness.
