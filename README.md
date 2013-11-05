## Sith

Sith is a black noise generator based on Spotify Core.

This is a work in progress.

> This product uses SPOTIFY CORE but is not endorsed, certified or otherwise
> approved in any way by Spotify. Spotify is the registered trade mark of the
> Spotify Group.

## Hacking

Request your own API key from https://developer.spotify.com/ and download the
latest version of libspotify to your system.

For now you will need to specify the password through the command line.

    $ go install github.com/op/sith
    $ sith -key path/app.key -username user -password pass
