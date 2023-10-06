# Forge Mod Update links for CurseForge Projects.

Enable the built-in Forge Update system with your mod with no code changes or hosting of websites. By using the power of
CurseForge, let us manage the JSON so you can focus on creating your mods!

## API

The CurseForge UpdateJson API is available at https://curseupdate.com. Please also refer here for the full documentation

`GET https://curseupdate.com/{projectId}/{modid}?ml={loader}`

Project IDs can be found by going to your project on CurseForge and looking for the "Project ID" on the right. 
The mod id is your modid from the mods.toml file.
Loader is the loader for the mod, including (but not limited to) forge, fabric, and neoforge.

Alternatively, the loader may be passed using the hostname, but this is only supported for the following. These calls
will not require the `ml` parameter to be passed. If both are used, the `ml` parameter will have priority.

    - forge.curseupdate.com
    - fabric.curseupdate.com
    - neoforge.curseupdate.com
    - quilt.curseupdate.com

We query CurseForge and retrieve all versions of your mod. We parse each of your files and pull the versions from your
mods.toml. Using this, we construct the appropriate JSON structure needed for Forge's update checker. This means we
provide the most accurate versions of your mod for Forge to look at. We do not rely on specific file names or
structures, and can support all the mod ids in your mods.

Example:

`GET https://curseupdate.com/32274/journeymap?ml=forge`

```json
{
  "promos": {
    "1.14.4-latest": "5.7.0",
    "1.14.4-recommended": "5.7.0",
    "1.15.2-latest": "5.7.0",
    "1.15.2-recommended": "5.7.0",
    "1.16.2-latest": "5.7.3",
    "1.16.2-recommended": "5.7.3",
    "1.16.3-latest": "5.7.3",
    "1.16.3-recommended": "5.7.3",
    "1.16.4-latest": "5.7.3",
    "1.16.4-recommended": "5.7.3",
    "1.16.5-latest": "5.8.0beta1",
    "1.16.5-recommended": "5.7.3",
    "1.17.1-latest": "5.8.0beta1",
    "1.17.1-recommended": "5.7.3",
    "1.18-latest": "5.8.0alpha3"
  },
  "homepage": "https://www.curseforge.com/minecraft/mc-mods/journeymap"
}
```

`GET https://curseupdate.com/32274/journeymap/references?ml=forge`

```json
{
  "1.14.4-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3208023",
  "1.14.4-recommended": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3208023",
  "1.15.2-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3208019",
  "1.15.2-recommended": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3208019",
  "1.16.2-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3397059",
  "1.16.2-recommended": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3397059",
  "1.16.3-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3397059",
  "1.16.3-recommended": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3397059",
  "1.16.4-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3397059",
  "1.16.4-recommended": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3397059",
  "1.16.5-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3640445",
  "1.16.5-recommended": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3397059",
  "1.17.1-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3640443",
  "1.17.1-recommended": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3509575",
  "1.18.1-latest": "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/3640441"
}
```      

For non-Forge mods, there is no defined structure offered by other teams. As such, we return the same JSON structure. To
get data for those other mods, you can pass the modloader slug name (located next to the full name in the list below) to
the URL in the ml query param. This list may change at any time based on CurseForge, however known ones are:

    - Fabric (fabric)
    - Quilt (quilt)
    - NeoForge (neoforge)
    - Forge (forge)
    - Rift (rift)
    - Risugami's ModLoader (risugamis-modloader)

## Responses

Each request response is a JSON document containing either project version data or an error.

| Status | Description                                                                                                                                                                  |
|--------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 200    | Project found and JSON is attached                                                                                                                                           |
| 400    | Path provided is not a valid CurseForge ID                                                                                                                                   |
| 404    | Path is valid but CurseForge responded that no project exists. This may be because no project has ever lived at this path, or a project does but is hidden from public view. |
| 500    | An unknown error occurred processing your request.                                                                                                                           |

## Project Data

Data is pulled from CurseForge directly, with mod versions cached locally to avoid heavy pulls from CurseForge. This
does mean that if we have not seen a project before, the first call will be slow, as we have to pull files from
CurseForge to populate this cache, but future updates and calls will be served quickly.

Each time a new file is released on CurseForge and a call is made to this service, we pull the new file and analyze it
to update the json.

## Cache

All URL calls are cached for 5 minutes on the backend server, so that repeated calls to the same URL do not overload
CurseForge with requests, and can be quickly served to end users. In the event this cache needs to be purged for a
specific URL, /expire can be added to the URL to force it to be expired. Do note this is only meant for updating the
JSON immediately after a release. Abuse of this feature will result in it being removed.

## Contact

We now have a Discord! - https://discord.gg/FENdtjAJRF

Security messages may be sent to admin (at) cfwidget.com

## Privacy

All traffic is routed through Cloudflare, which follows the Cloudflare Privacy Policy. No identifiable information is
logged or stored on the backend servers.
