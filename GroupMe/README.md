# GroupMe support is unfinished

This is just a stub, it is buggy and untested. I have no intention of finishing it unless there is a real need for it. It is just here
as an example of how to write a messaging interface.

Use the developer interface to create a bot and add it to a room. This code does not establish a new bot (because that was just bad, check git if you want to see how to do it, but I don't recommend it). This does not update on additional bots being added. Restart the wasabi process to catch those changes (fix this if you want do do more than test this).

This does not SEND messages since I am not storing gid<->gm mappings. This could easily be changed.

You cannot (in the v3 API) send direct messages to the bot, which makes it much less useful for our purposes. The only thing to do is create a room, add the bot to the room, then send location info (or whatever) to the room--which is janky. I wouldn't recommend using GroupMe for anything other than sending messages to users as a kind of last-ditch method.
