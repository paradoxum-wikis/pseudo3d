# The Manual

In order to use this toolset, look at the ["Releases"](https://github.com/paradoxum-wikis/pseudo3d/releases) tab and download the latest build, then extract the archive to a location of your choice.

The archives available for downloads are:
 - **pseudo3d-sprites-windows-amd64.zip**: Windows 64-bit.
 - **pseudo3d-sprites-linux-amd64.tar.xz**: Linux 64-bit.
 - **pseudo3d-sprites-macos-amd64.zip**: MacOS 64-bit.
 - **pseudo3d-sprites-macos-arm64.zip**: MacOS Apple Silicon.

## Setting Up the Toolset

1. Open the folder in your file explorer, and you will see a "Main.rbxmx" file.
2. Drag and drop that file into your Roblox Studio.
3. Head to the explorer window, and move the newly added "Main" into `StarterPlayer` > `StarterPlayerScripts`.

## Capturing the Frames

Now that you've set up the toolset, you can start using it. I will not go over how to get the models or animations, since you're expected to be the one providing those.

1. First, get the model you want and place it in your `Workspace`, **make sure to rename the model to "Target."**
2. Next, get the animation **keyframe** (`KeyframeSequence`), and move it inside the root level of the model. Rename the keyframe to "Animation".
Your model should now look something like this:
```
Target (Model)
	├─ Animation (KeyframeSequence)
	└─ Random Garbage (Idfk)
```

You can run the game now (F5), and the toolset will automatically angle your camera and whatnot. There are control guides at the top of your screen to help you as well.

There are also more configurations you can change inside your `LocalScript`, such as the backdrop color, etc., but for most, if not all, use cases on our wikis, it isn't necessary to change any of those.

Press Q and you can start the capturing process. Note that your cursor **will be hidden**, so you will have to estimate where your cursor is when you want to click "Save."

**All of your images will be saved in a folder called "tmp-capture-storage."** You can find this folder by:
- On Windows: Windows + R > `%localappdata%\Roblox\tmp-capture-storage` > Enter.
- On MacOS: Finder > Command + Shift + G > `~/Library/Application Support/Roblox/tmp-capture-storage` > Enter.
- On Linux (Vinegar):
	- Flatpak: `cd ~/.var/app/org.vinegarhq.Vinegar/data/vinegar/studio/prefix/drive_c/users/$USER/AppData/Local/Roblox/tmp-capture-storage`
	- Native: `cd ~/.local/share/vinegar/studio/prefix/drive_c/users/$USER/AppData/Local/Roblox/tmp-capture-storage`

You will have to move all of the files you've just captured (24 by default) to the "process" folder inside your toolset directory.

## Processing the Frames

After you've captured the frames and moved them into the "process" folder, you can start processing them. Simply run the executable file inside the folder, and follow the instructions.

> [!NOTE]
> The program run may be flagged by Windows SmartScreen for not having a verified publisher; this is a false positive, as I do not have the resources to get a code signing certificate.
>
> If it were actually malware, Windows Defender would have blocked the file instead of showing a warning. Besides, the source code is open, and you can verify that yourself.

The UI will show you one of your captured frames. You can slide through them all with the slider below the image. Now, simply draw a square around the area you want to keep, then click "Save." In case a square won't fit the area you want to capture, you can click on the button that says "Square Ratio" to change it to "Freeform."

The program will close the UI and immediately begin batch processing all frames in the "process" folder; the progress will be shown in your terminal. Once it's done, you'll find your spritesheet saved as `spritesheet.png` in the root directory of the repository.

## Flags Reference

For advanced users, there are some additional options you can configure to customize the processing behavior. These options are passed as flags when running the executable through a terminal. For example:

Let's say, if you've already configured your safe zone and just want to re-run the batch processing without opening the web UI again, maybe because the previous run wasn't up to your liking, you can use the `-process` flag:

```
pseudo3d-sprites.exe -skip-ui
```

Or, maybe you want to limit the output size of each frame to 256×256 and increase the background removal to be more aggressive. You can run:

```
pseudo3d-sprites.exe -size 256 -threshold-bg 80
```

Here is a full list of available flags:

| Flag | Default | Description |
|---|---|---|
| `-skip-ui` | `false` | Skip the UI and run batch processing directly using the saved safe zone. |
| `-size` | `512` | Output size of each square frame in pixels (e.g., `300` for 300×300). |
| `-skip-bg` | `false` | Skip chroma key background removal entirely. |
| `-threshold-bg` | `70.0` | Tolerance for background removal (higher values are more aggressive). |
| `-in` | `./process` | Input directory containing your PNG frames. |
| `-out` | `spritesheet.png` | Output filename for the final spritesheet. |
| `-erode` | `false` | Trim 1 pixel of alpha from edges to remove chroma key residue. |
| `-color-bg` | `DF03DF` | Hex color to remove as the background (should match `BACKDROP_COLOR` in your "Main" `LocalScript`). |
| `-skip-prescale` | `false` | Prescale preview images to make the UI perform better; the original files are still used for processing. |

*You can also access the above list by running the executable with the `-help` (`-h`) flag.*
