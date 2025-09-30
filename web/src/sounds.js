class SoundManager {
    constructor() {
        this.audioContext = null;
        this.enabled = true;
        this.initAudioContext();
    }

    initAudioContext() {
        try {
            window.AudioContext = window.AudioContext || window.webkitAudioContext;
            this.audioContext = new AudioContext();
        } catch (e) {
            console.warn('Web Audio API not supported', e);
            this.enabled = false;
        }
    }

    resumeContext() {
        if (this.audioContext && this.audioContext.state === 'suspended') {
            return this.audioContext.resume();
        }

        return Promise.resolve();
    }

    playTone(frequency, duration, type = 'sine', volume = 0.3) {
        if (!this.enabled || !this.audioContext) return;

        return this.resumeContext().then(() => {
            const oscillator = this.audioContext.createOscillator();
            const gainNode = this.audioContext.createGain();

            oscillator.connect(gainNode);
            gainNode.connect(this.audioContext.destination);

            oscillator.frequency.value = frequency;
            oscillator.type = type;

            gainNode.gain.setValueAtTime(volume, this.audioContext.currentTime);
            gainNode.gain.exponentialRampToValueAtTime(0.01, this.audioContext.currentTime + duration);

            oscillator.start(this.audioContext.currentTime);
            oscillator.stop(this.audioContext.currentTime + duration);
        });
    }

    playMarioVictory() {
        if (!this.enabled || !this.audioContext) return;

        return this.resumeContext().then(() => {
          const notes = [
            // Step 1: G major arpeggio near G5
            { freq: 783.99, duration: 0.05, delay: 0.00 },  // G5
            { freq: 987.77, duration: 0.05, delay: 0.05 },  // B5
            { freq: 1174.66, duration: 0.05, delay: 0.10 }, // D6

            // Step 2: C6 chord arpeggio
            { freq: 1046.50, duration: 0.05, delay: 0.15 }, // C6
            { freq: 1318.51, duration: 0.05, delay: 0.20 }, // E6
            { freq: 1568.00, duration: 0.05, delay: 0.25 }, // G6

            // Step 3: E6 chord arpeggio
            { freq: 1318.51, duration: 0.05, delay: 0.30 }, // E6
            { freq: 1661.22, duration: 0.05, delay: 0.35 }, // G#6
            { freq: 1975.53, duration: 0.05, delay: 0.40 }, // B6

            // Step 4: G6 chord arpeggio
            { freq: 1568.00, duration: 0.05, delay: 0.45 }, // G6
            { freq: 1975.53, duration: 0.05, delay: 0.50 }, // B6
            { freq: 2349.32, duration: 0.05, delay: 0.55 }, // D7

            // Step 5: C7 chord arpeggio
            { freq: 2093.00, duration: 0.05, delay: 0.60 }, // C7
            { freq: 2637.02, duration: 0.05, delay: 0.65 }, // E7
            { freq: 3135.96, duration: 0.05, delay: 0.70 }, // G7

            // Step 6: G6 chord arpeggio
            { freq: 1568.00, duration: 0.05, delay: 0.75 }, // G6
            { freq: 1975.53, duration: 0.05, delay: 0.80 }, // B6
            { freq: 2349.32, duration: 0.05, delay: 0.85 }, // D7

            // Step 7: C7 sustain (hold)
            { freq: 2093.00, duration: 0.50, delay: 0.90 }  // C7
          ];

            notes.forEach(note => {
                setTimeout(() => {
                    this.playTone(note.freq, note.duration, 'square', 0.18);
                }, note.delay * 1000);
            });
        });
    }

    playSuccess() {
        if (!this.enabled || !this.audioContext) return;

        return this.resumeContext().then(() => {
            const notes = [
              { freq: 783.99, duration: 0.15, delay: 0 },
              { freq: 1046.50, duration: 0.15, delay: 0.15 },
              { freq: 1318.51, duration: 0.15, delay: 0.30 },
              { freq: 1568.00, duration: 0.15, delay: 0.45 },
              { freq: 2093.00, duration: 0.15, delay: 0.60 },
              { freq: 1568.00, duration: 0.15, delay: 0.75 },
              { freq: 2093.00, duration: 0.5, delay: 0.90 }
            ];

            notes.forEach(note => {
                setTimeout(() => {
                    this.playTone(note.freq, note.duration, 'sine', 0.15);
                }, note.delay * 1000);
            });
        });
    }

    playPowerUp() {
        if (!this.enabled || !this.audioContext) return;

        return this.resumeContext().then(() => {
            const startFreq = 200;
            const endFreq = 800;
            const duration = 0.3;

            const oscillator = this.audioContext.createOscillator();
            const gainNode = this.audioContext.createGain();

            oscillator.connect(gainNode);
            gainNode.connect(this.audioContext.destination);

            oscillator.frequency.setValueAtTime(startFreq, this.audioContext.currentTime);
            oscillator.frequency.exponentialRampToValueAtTime(endFreq, this.audioContext.currentTime + duration);
            oscillator.type = 'sawtooth';

            gainNode.gain.setValueAtTime(0.15, this.audioContext.currentTime);
            gainNode.gain.exponentialRampToValueAtTime(0.01, this.audioContext.currentTime + duration);

            oscillator.start(this.audioContext.currentTime);
            oscillator.stop(this.audioContext.currentTime + duration);
        });
    }

    playCoin() {
        if (!this.enabled || !this.audioContext) return;

        return this.resumeContext().then(() => {
            const notes = [
                { freq: 987.77, duration: 0.1, delay: 0 },
                { freq: 1318.51, duration: 0.4, delay: 0.1 }
            ];

            notes.forEach(note => {
                setTimeout(() => {
                    this.playTone(note.freq, note.duration, 'square', 0.15);
                }, note.delay * 1000);
            });
        });
    }

    playFirework() {
        if (!this.enabled || !this.audioContext) return;

        return this.resumeContext().then(() => {
            const whiteNoise = this.audioContext.createBufferSource();
            const bufferSize = this.audioContext.sampleRate * 0.3;
            const buffer = this.audioContext.createBuffer(1, bufferSize, this.audioContext.sampleRate);
            const data = buffer.getChannelData(0);

            for (let i = 0; i < bufferSize; i++) {
                data[i] = Math.random() * 2 - 1;
            }

            whiteNoise.buffer = buffer;

            const gainNode = this.audioContext.createGain();
            const filter = this.audioContext.createBiquadFilter();

            filter.type = 'highpass';
            filter.frequency.value = 1000;

            whiteNoise.connect(filter);
            filter.connect(gainNode);
            gainNode.connect(this.audioContext.destination);

            gainNode.gain.setValueAtTime(0.1, this.audioContext.currentTime);
            gainNode.gain.exponentialRampToValueAtTime(0.01, this.audioContext.currentTime + 0.3);

            whiteNoise.start(this.audioContext.currentTime);
            whiteNoise.stop(this.audioContext.currentTime + 0.3);
        });
    }

    disable() {
        this.enabled = false;
    }

    enable() {
        this.enabled = true;
        if (!this.audioContext) {
            this.initAudioContext();
        }
    }
}

window.soundManager = new SoundManager();
