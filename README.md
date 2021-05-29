# outgoing-webhook-server
It receives outgoing webhook and call specified command.

## Environment Variables

* WEBHOOK_CMD (required)
   * command
* WEBHOOK_PORT (default:22999)
   * Listen Port
* WEBHOOK_BIND (default:0.0.0.0)
   * Bind IP
* WEBHOOK_PATH (default:/)
   * Endpoint PATH
* WEBHOOK_DEBUG (defualt:false)
   * Enable debug mode
* WEBHOOK_LOG_PREFIX (default:[webhook])
   * Log prefix
* WEBHOOK_NO_ALOG (default:false)
   * Suppress access log

## Usage

Simple 

```
$ WEBHOOK_CMD="./my-script.sh" ./webhook 
```

Listening on 192.168.0.1:8080

```
$ WEBHOOK_PORT="192.168.0.1" WEBHOOK_PORT="8080" WEBHOOK_CMD="./my-script.sh" ./webhook 
```

Change log prefix from [webhook] to myapp

```
$ WEBHOOK_LOG_PREFIX="myapp" WEBHOOK_CMD="./my-script.sh" ./webhook 
```

