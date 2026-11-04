# YouTube artist and title metadata

YouTube stations use yt-dlp to fetch metadata **without downloading video or audio**.
Add a video in **Admin → Music library → Add YouTube video**. The server reads
the video's title, removes descriptive suffix tags, separates the artist/title,
and persists the result in `/data/track-metadata.json`. This requires a
writable persistent `/data` volume.

Example:

| Input YouTube title | Artist | Title |
| --- | --- | --- |
| `THIRST - MAYHEM [Drift Phonk \| Monstercat]` | `THIRST` | `MAYHEM` |
| `Artist – Song [Official Audio]` | `Artist` | `Song` |

The parser removes **trailing square brackets** repeatedly, along with
recognised promotional suffixes such as `(Official Music Video)`,
`(Lyrics)`, `(Visualizer)`, and `(HD)`. It will not remove
bracketed text *inside* a title.

When no `Artist - Song` separator exists, yt-dlp's structured `artist`
or `creator` property is used if available. The YouTube uploader is not
assumed to be the artist. If the metadata is unavailable, the link can still
be added and the admin will display a notice instead of inventing metadata.

The cleaned values feed the station library, ICY now-playing metadata,
`/api/v1/radios`, and the VRChat HLS title overlay on upcoming song
transitions. No audio stream restart is required.

Existing YouTube videos lacking cached metadata are looked up in the
background after server start, with a 15-second maximum per lookup, without
blocking radio startup. Successful results are written to persistent storage,
so they are not repeatedly retrieved on every reboot.

A **Refresh YouTube title** button is available beside each video in Manage
Music. Refreshing explicitly overwrites manually edited metadata for that
video. The original, unaltered YouTube video title is preserved separately as
`source_title` for reference.

The existing `Edit metadata` control remains available for exceptions where
automatic parsing cannot determine the artist/title reliably.

For public radio, supply only video/music you are authorised to rebroadcast.
Metadata access is not permission to rebroadcast the source media.
