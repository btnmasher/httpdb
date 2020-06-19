# httpdb

A small key/value store REST service. It is implemented to be as resilient to race conditions as possible, utilizing carefully placed `sync.Mutex` locks in key areas. I have not tested it with a very large load of requests to verify this, but so far so good!

## Usage

Operations on the mini database are done via RESTful HTTP calls.

#### Key Components

- `{key}` - A unique string used as an ID for a particular value entry.
- `{lock_id}` - A unique string used as an identifier of an exclusive lock on a particular `{key}`

#### Configuration

The configuration file is formatting in JSON, and is attempted to be read at the default locaiton of the application directory with the filename `httpdb.conf.json`.

###### Command-line Flags

- The `--config=/path/to/conf.json` command-line flag exists to specify the path to the configuration file. The path is considered to be relative if the leading `/` is not specified.

- The `--showconf` command-line flag exists to have the application attempt to load and parse the configuration file then print it to the console and exit with status `1`. Will print out the default configuration if the load/parse of the configuration file fails.

###### Configuration File Format

- `"port"` - `integer` - The port to listen for HTTP requests on.
- `"debug"` - `bool` - Turn on `DEBUG` level logging.
- `"timeout"` - `integer` - The number of `time.Second` to wait when attempting to acquire a lock on an existing `{key}` that already holds a lock before timing out the request.
- `"atomic_buffer"` - `integer` - The internal buffer for all atomic actions on `Entry` objects. The higher this is set, the higher the number of atomic actions can be performed without blocking other requests.

Example configuration (these are the application defaults in the event of a missing configuration file):

```JSON
{
    "app": {
        "port": 9000,
        "debug": false,
        "timeout": 5,
    "atomic_buffer": 100
    }
}
```

### REST API

___

### `POST /reservations/{key}`

- If `{key}` doesn't exist, returns `404 Not Found`.
- if `{key}` exists and is not locked, acquires the lock, returns `200 OK`, the `{key}`'s value and a new `{lock_id}`
- If `{key}` exists, and it is locked, waits until the lock is available for the configured period of time (`Config.App.Timeout * time.Second`). If unsuccessful, returns `408 Requeset Timeout`.
- If `{key}` exists and is locked, waits until the lock is available for the configured period of time (`Config.App.Timeout * time.Second`). If successfull, acquires the lock, returns `200 OK`, the `{key}`'s value, and a new `{lock_id}`.

Returns the value of `{key}` along with a unique `{lock_id}` that the caller can use in later calls.

The response body should be `application/json` in the form of:

```json
{
  "value": "something",
  "lock_id": "something_else"
}
```

___

### `POST /values/{key}/{lock_id}?release={true, false}`

Attempt to update the value of `{key}` to the value given in the `POST` body according to
the following rules:

- If `{key}` doesn't exist, returns `404 Not Found`
- If `{key}` exists but `{lock_id}` doesn't identify the currently held lock (or if there is no lock), does no action and responds immediately with `401 Unauthorized`.
- If `{key}` exists, `{lock_id}` identifies the currently held lock and `release=true`, sets the new value, releases the lock and invalidates `{lock_id}`. Returns `204 No Content`
- If `{key}` exists, `{lock_id}` identifies the currently held lock and `release=false`, sets the new value but doesn't release the lock and keeps `{lock_id}` valid. Returns `204 No Content`
- In all cases, `release={true, false}` query value is considered false if it is omitted from the request path.

___

### `PUT /values/{key}`

- If `{key}` doesn't already exist, create it and immediately acquire the lock on it, returns `200 OK` and a new `{lock_id}`
- If `{key}` already exists, and it is locked, waits until the lock is available for the configured period of time (`Config.App.Timeout * time.Second`). If unsuccessful, returns `408 Requeset Timeout`.
- If `{key}` already exists, and it is locked, waits until the lock is available for the configured period of time (`Config.App.Timeout * time.Second`). If successfull, overwrites the `{key}`'s value with the new data in `PUT` body, then returns `200 OK` and a new `{lock_id}`.

In both successful cases, returns the new `{lock_id}` in the form of:

```json
{
  "lock_id": "abc"
}
```

### Testing

The `handlers_test.go` file contains a small set of tests.

You can run them with `go test -v -race` which will also watch for race conditions.

## Why?

The original idea for this piece of software came from a [programming exercise](https://github.com/arschles/go-progprobs/blob/master/minidb.md). (All credits to Aaron Schlesinger from Iron.io) I was tasked with implementing it for a technical interview, and I decided to make it as best as I could (with very minor adjustments), and release it open source! Will I ever have any use for it? Maybe. Will you? *shrug* It's dirty, it needs better patterns, it probably isn't idomatic, but it's mine so that's all that matters :)

## Todo

- Load testing
  - Run all the handler tests in high volume to determine if our operations aren't racing at high request speeds.
  - Determine a good default for the atomic buffer.

## Contribute

Send me a pull request/issue, same as always.
