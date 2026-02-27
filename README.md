# The Manual

> [!NOTE]
> I am currently working on a prebuilt version of this project, which will be available for download soon.
> In the meantime, you can follow the instructions below to set up the project on your local machine.

In order to use this project, you need to have Go, Git (or GitHub Desktop), Roblox Studio, and a browser. Most likely, you already have the latter two, but you may need to install Go and Git.

- You can download Go from the official website: https://golang.org/dl/.
- For Git, **I recommend using GitHub Desktop**, which provides a beginner-friendly interface. You can download it from: https://desktop.github.com/download/.
- If you prefer using the command line, you can download Git from the official website: https://git-scm.com/install/.

## Setting Up the Toolset

The rest of the setup will assume you are using GitHub Desktop (because otherwise I doubt you'd be reading this):

1. Find the green "Code" button on the top right of the repository page and click it.
2. Select "Open with GitHub Desktop" from the dropdown menu.
3. GitHub Desktop will open, just follow the prompts to clone the repository to your local machine.
4. Once the repository is cloned, open the folder in your file explorer and you will see a "main.luau" file.
5. Open your Studio, head to the explorer and create a `LocalScript` inside `StarterPlayer` > `StarterPlayerScripts`.
6. Finally, copy the contents of "main.luau" and paste it into the LocalScript you just created.

## Capturing the Frames

Now that you've set up the toolset, you can start using it. I will not go over how to get the models or animations, since you're expected to be the one providing those.

1. First get the model you want and place it in your `Workspace`, **make sure to rename the model to "Target"**.
2. Next, get the animation **keyframe** (`KeyframeSequence`), and move it inside the root level of the model. Rename the keyframe to "Animation".
Your model should now look something like this:
```
Target (Model)
	├─ Animation (KeyframeSequence)
	└─ Random Garbage (Idfk)
```

You can run the game now (F5) and the toolset will automatically angle your camera and whatnot, there are control guides on the top of your screen to help you as well.

There are also more configurations you can change inside your `LocalScript`, such as the backdrop color and etc., but for most if not all use cases on our wikis, it isn't necessary to change any of those.

Press Q and you can start the capturing process, note that your cursor **will be hidden**, so you will have to estimate where your cursor is when you want to click "Save."

**All of your images will be saved in a folder called `tmp-capture-storage`.** You can find this folder by:
- On Windows: Windows + R > `%localappdata%\Roblox\tmp-capture-storage` > Enter.
- On MacOS: Finder > Command + Shift + G > `~/Library/Application Support/Roblox/tmp-capture-storage` > Enter.
- On Linux (Vinegar):
	- Flatpak: `cd ~/.var/app/org.vinegarhq.Vinegar/data/vinegar/studio/prefix/drive_c/users/$USER/AppData/Local/Roblox/tmp-capture-storage`
	- Native: `cd ~/.local/share/vinegar/studio/prefix/drive_c/users/$USER/AppData/Local/Roblox/tmp-capture-storage`

You will have to move all of the files you've just captured (24 by default) to the "process" folder inside your local repository folder.

## Processing the Frames

Now that you've captured the frames and moved them into the "process" folder, you can start processing them.

First, make sure you are in the **root directory of the repository** in your terminal, and not inside any folders such as "process" or "archive."

In your file explorer, you can right click on the folder itself and select "Open in Terminal" (or "Open in Command Prompt", whatever is available). This will open a terminal window with the current directory set to the folder you just right-clicked on.

Type "go run main.go" and press Enter to run the Go program. This will ask you to allow it to access your network, which is required since it is opening a local server so that it can serve you a web UI for selecting the **safe zone** - the region of the frame that will be cropped and used in the final spritesheet. Allow the access, then open http://localhost:7777 in your browser.

The web UI will show you one of your captured frames, you can slide through them all with the slider below the image. Now, simply draw a square around the area you want to keep, then click "Save." In case a square won't fit the area you want to capture, you can click on the button that says "Square Ratio" to change it to "Freeform."

The program will close the web server and immediately begin batch processing all frames in the "process" folder, the progress will be shown in your terminal. Once it's done, you'll find your spritesheet saved as `spritesheet.png` in the root directory of the repository.

## Flags Reference

You can customize the processing behavior by passing flags to the `go run main.go` command. For example:

```
go run main.go -size 256 -threshold 80
```

If you've already configured your safe zone and just want to re-run the batch processing without opening the web UI again, maybe because the previous run wasn't up to your liking, you can use the `-process` flag:

```
go run main.go -process
```

Here is a full list of available flags:

| Flag | Default | Description |
|---|---|---|
| `-process` | `false` | Skip the web UI and run batch processing directly using the saved safe zone |
| `-size` | `512` | Output size of each square frame in pixels (e.g. `300` for 300×300) |
| `-no-bg` | `false` | Skip chroma key background removal entirely |
| `-threshold` | `70.0` | Tolerance for background removal — higher values are more aggressive |
| `-in` | `./process` | Input directory containing your PNG frames |
| `-out` | `spritesheet.png` | Output filename for the final spritesheet |
| `-erode` | `false` | Trim 1 pixel of alpha from edges to remove chroma key residue |
| `-color` | `DF03DF` | Hex color to remove as the background (should match `BACKDROP_COLOR` in your LocalScript) |
