Wordle TUI app, written in Go, stylized like monkeytype, with Bubble Tea.

press tab and enter to generate new puzzle

if theres any progress on any puzzle, save it as Wordle #(ID). there will be a list of attempted wordles, finished or not, in the app, that you can go back to and review if solved, and solve if unsolved.

id can be UUID or something else. 

should have monkeytype like profile section, with leaderboard and general performance metrics, we’ll get more ideas for it later. so dont bottle yourself in with any of this in this doc. one metric that i can suggest is average number of attempts to solve a wordle and average time taken

Add categories for 5 letter words, 6 letter words and 4 letter words, as the main 3. (like 15, 30, 60 in monkeytype)

after this project is solidified, i aim to make this global, like w a leaderboard and such. will have a central server somewhere so people can “sign in” w an id (don’t know how this might work, auth seems like too much?)

Very rudimentary plan, everything might not be defined. so ask questions if needed, or declare your assumptions. dont box yourself with a very tight architecture, as scope might widen a lot after.
