package transcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
 "cliamp-server/s3store"
 "cliamp-server/config"
 "strings"
)

const (
	// OutputBitrate is the station MP3 bitrate in kilobits per second.
	OutputBitrate = 128
	// OutputSampleRate is the station sample rate in hertz.
	OutputSampleRate = 44100
	// OutputChannels is the number of station audio channels.
	OutputChannels = 2
	// PCMBytesPerSample is the size of one signed 16-bit PCM sample.
	PCMBytesPerSample = 2
	// PCMFrameBytes is the size of one interleaved PCM sample frame.
	PCMFrameBytes = OutputChannels * PCMBytesPerSample

	// EncoderDelaySamples is libmp3lame's delay as exposed by FFmpeg. Track
	// transitions use it to align metadata with the continuously encoded audio.
	EncoderDelaySamples = 1105
)

// NewReader starts FFmpeg and returns a fixed-profile MP3 stream for path.
// It is used for per-listener audio such as station intros.
func NewReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return startReader(ctx,
		"-hide_banner",
		"-loglevel", "error",
		"-nostats",
		"-nostdin",
		"-i", path,
		"-map", "0:a:0",
		"-vn",
		"-sn",
		"-dn",
		"-c:a", "libmp3lame",
		"-b:a", fmt.Sprintf("%dk", OutputBitrate),
		"-ar", fmt.Sprintf("%d", OutputSampleRate),
		"-ac", fmt.Sprintf("%d", OutputChannels),
		"-id3v2_version", "0",
		"-write_id3v1", "0",
		"-write_xing", "0",
		"-flush_packets", "1",
		"-f", "mp3",
		"pipe:1",
	)
}

// NewPCMReader starts FFmpeg and decodes path to the station's raw PCM profile.
func NewPCMReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return startReader(ctx,
		"-hide_banner",
		"-loglevel", "error",
		"-nostats",
		"-nostdin",
		"-i", path,
		"-map", "0:a:0",
		"-vn",
		"-sn",
		"-dn",
		"-c:a", "pcm_s16le",
		"-ar", fmt.Sprintf("%d", OutputSampleRate),
		"-ac", fmt.Sprintf("%d", OutputChannels),
		"-f", "s16le",
		"pipe:1",
	)
}

func startReader(ctx context.Context, args ...string) (io.ReadCloser, error) {
 for i:=0;i+1<len(args);i++ {
 if args[i]!="-i"{continue}
 input:=args[i+1]
 if input=="silence://standby"{
  replacement:=[]string{"-f","lavfi","-i","anullsrc=channel_layout=stereo:sample_rate=44100","-t","5"}
  args=append(append(append([]string{},args[:i]...),replacement...),args[i+2:]...)
  break
 }
 if strings.HasPrefix(input,"youtube:") {
  // yt-dlp handles its own authenticated/anti-bot headers and fresh media
  // URLs. Its media bytes are piped straight into FFmpeg: the signed
  // googlevideo URL is never handed off to a separate HTTP client.
  return startYouTubeReader(ctx,strings.TrimPrefix(input,"youtube:"),args,i)
 }

 resolved,err:=s3store.Resolve(input);if err!=nil{return nil,err};args[i+1]=resolved
}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open ffmpeg output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("start ffmpeg: %w", err), stdout.Close())
	}

	return &processReader{cmd: cmd, pipe: stdout}, nil
}

func startYouTubeReader(ctx context.Context,videoURL string,args []string,inputIndex int)(io.ReadCloser,error){
 // --output - streams to stdout; --get-url is intentionally not used.
 // Limit this path to a single video and never accept arbitrary URLs.
 if !config.ValidYouTubeURL(videoURL){return nil,errors.New("unsupported YouTube video URL")}
 download:=exec.CommandContext(ctx,"yt-dlp",
  "--ignore-config","--no-playlist","--no-warnings","--quiet",
  "--js-runtimes","node","--format","bestaudio/best",
  "--output","-",videoURL)
 // Avoid logging signed origin URLs or credential-like query parameters.
 download.Stderr=io.Discard
 media,err:=download.StdoutPipe()
 if err!=nil{return nil,fmt.Errorf("open YouTube media pipe: %w",err)}
 if err:=download.Start();err!=nil{_ = media.Close();return nil,fmt.Errorf("start yt-dlp media transfer: %w",err)}
 ffArgs:=append([]string(nil),args...)
 ffArgs[inputIndex+1]="pipe:0"
 ff:=exec.CommandContext(ctx,"ffmpeg",ffArgs...)
 ff.Stdin=media
 ff.Stderr=os.Stderr
 stdout,err:=ff.StdoutPipe()
 if err!=nil{
  _=download.Process.Kill();_ = media.Close();_ = download.Wait()
  return nil,fmt.Errorf("open FFmpeg decoded audio: %w",err)
 }
 if err:=ff.Start();err!=nil{
  _=download.Process.Kill();_ = media.Close();_ = download.Wait();_ = stdout.Close()
  return nil,fmt.Errorf("start FFmpeg YouTube audio decoder: %w",err)
 }
 return &processReader{cmd:ff,pipe:stdout,upstream:download,upstreamOutput:media},nil
}

type processReader struct {
	cmd  *exec.Cmd
	pipe io.ReadCloser
 upstream *exec.Cmd
 upstreamOutput io.ReadCloser

	waitOnce  sync.Once
	waitErr   error
	closeOnce sync.Once
	closeErr  error
}

func (r *processReader) Read(p []byte) (int, error) {
	n, err := r.pipe.Read(p)
	if err == nil {
		return n, nil
	}

	if waitErr := r.wait(); waitErr != nil {
		return n, fmt.Errorf("ffmpeg: %w", waitErr)
	}
	return n, err
}

func (r *processReader) Close() error {
	r.closeOnce.Do(func() {
		pipeErr := r.pipe.Close()
		if errors.Is(pipeErr, os.ErrClosed) {
			pipeErr = nil
		}

		var killErr error
		killed := false
		if r.cmd.Process != nil && r.cmd.ProcessState == nil {
			killErr = r.cmd.Process.Kill()
			if killErr == nil {
				killed = true
			} else if errors.Is(killErr, os.ErrProcessDone) {
				killErr = nil
			}
		}

		waitErr := r.wait()
		if killed {
			waitErr = nil
		}
		r.closeErr = errors.Join(pipeErr, killErr, waitErr)
	})
	return r.closeErr
}

func (r *processReader) wait() error {
	r.waitOnce.Do(func() {
		r.waitErr = r.cmd.Wait()
  if r.upstream!=nil{
   // FFmpeg is done reading media. Reap yt-dlp, including when the
   // decoder failed or the client disconnected early.
   if r.upstream.Process!=nil{_ = r.upstream.Process.Kill()}
   _=r.upstream.Wait()
   if r.upstreamOutput!=nil{_ = r.upstreamOutput.Close()}
  }
	})
	return r.waitErr
}

// Encoder continuously encodes the station PCM profile to MP3. Read and Write
// are intended to run concurrently so FFmpeg's pipes cannot deadlock.
type Encoder struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	closeInputOnce sync.Once
	closeInputErr  error
	waitOnce       sync.Once
	waitErr        error
}

var (
	_ io.Reader = (*Encoder)(nil)
	_ io.Writer = (*Encoder)(nil)
)

// NewEncoder starts one continuous encoder for a station.
func NewEncoder(ctx context.Context) (*Encoder, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-nostats",
		"-nostdin",
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", OutputSampleRate),
		"-ac", fmt.Sprintf("%d", OutputChannels),
		"-i", "pipe:0",
		"-map", "0:a:0",
		"-c:a", "libmp3lame",
		"-b:a", fmt.Sprintf("%dk", OutputBitrate),
		"-ar", fmt.Sprintf("%d", OutputSampleRate),
		"-ac", fmt.Sprintf("%d", OutputChannels),
		"-id3v2_version", "0",
		"-write_id3v1", "0",
		"-write_xing", "0",
		"-flush_packets", "1",
		"-f", "mp3",
		"pipe:1",
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open encoder input: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open encoder output: %w", err), stdin.Close())
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("start encoder: %w", err), stdin.Close(), stdout.Close())
	}

	return &Encoder{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (e *Encoder) Read(p []byte) (int, error) {
	return e.stdout.Read(p)
}

func (e *Encoder) Write(p []byte) (int, error) {
	return e.stdin.Write(p)
}

// CloseInput closes the PCM input and allows FFmpeg to flush its final frames.
func (e *Encoder) CloseInput() error {
	e.closeInputOnce.Do(func() {
		e.closeInputErr = e.stdin.Close()
		if errors.Is(e.closeInputErr, os.ErrClosed) {
			e.closeInputErr = nil
		}
	})
	return e.closeInputErr
}

// Wait reaps the encoder process. Call it after the output reader has finished.
func (e *Encoder) Wait() error {
	e.waitOnce.Do(func() {
		e.waitErr = e.cmd.Wait()
	})
	return e.waitErr
}
