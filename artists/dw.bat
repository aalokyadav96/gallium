@echo off
set /p URL=Enter the signed URL: 
set /p FILENAME=Enter the filename (example: my_song.mp4): 

echo Downloading...
curl -L "%URL%" ^
  -H "Referer: https://www.jiosaavn.com/" ^
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36" ^
  -o "%FILENAME%"

echo Done! Saved as %FILENAME%
pause
