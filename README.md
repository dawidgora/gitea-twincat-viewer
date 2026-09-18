# gitea-twincat-viewer

`gitea-twincat-viewer` is a pure Go Gitea external renderer for Beckhoff
TwinCAT files. It turns the Structured Text in `POU`, `DUT`, and `GVL` files
into readable, syntax-highlighted HTML through a single static binary.

## Screenshots

| Dark theme | Light theme |
| --- | --- |
| <a href="docs/images/function-block-dark.png"><img src="docs/images/function-block-dark.png" alt="Function block rendered in the dark theme" width="400"></a> | <a href="docs/images/function-block-light.png"><img src="docs/images/function-block-light.png" alt="Function block rendered in the light theme" width="400"></a> |
| <a href="docs/images/global-variables-dark.png"><img src="docs/images/global-variables-dark.png" alt="Global variables rendered in the dark theme" width="400"></a> | <a href="docs/images/global-variables-light.png"><img src="docs/images/global-variables-light.png" alt="Global variables rendered in the light theme" width="400"></a> |

## Supported TwinCAT files

| Extension | Kind | Rendered content |
| --- | --- | --- |
| `.TcPOU` | POU | Name, kind, declaration, implementation, methods, properties, and actions |
| `.TcDUT` | DUT | Name, kind, and declaration |
| `.TcGVL` | GVL | Name and global variable declarations |

## Gitea configuration

Configure the renderer through Gitea's `app.ini`. The simplest approach is to
mount an additional `.ini` file into `/etc/gitea/app.ini.d/`.

For example, `deploy/gitea/app.ini.d/twincat.ini` can contain:

```ini
[markup.twincat]
ENABLED = true
FILE_EXTENSIONS = .TcPOU,.TcDUT,.TcGVL
RENDER_COMMAND = /opt/gitea-extensions/gitea-twincat-viewer
IS_INPUT_FILE = true
RENDER_CONTENT_MODE = no-sanitizer
```
