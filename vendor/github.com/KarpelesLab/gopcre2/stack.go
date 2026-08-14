package gopcre2

// frameKind identifies the type of a backtrack stack frame.
type frameKind uint8

const (
	frameSplit      frameKind = iota // alternative branch to try
	frameCapture                     // saved capture slot
	frameLookaround                  // checkpoint for lookaround
	frameAtomic                      // cut point for atomic group
	frameRecursion                   // recursion return info
	frameVerb                        // backtracking verb marker
	frameMark                        // (*MARK:NAME) record
	frameFlagSave                    // saved flags state
)

// stackFrame is a single entry on the backtrack stack.
type stackFrame struct {
	kind       frameKind
	pc         uint32   // instruction to resume at (for frameSplit)
	pos        int      // subject position
	capSlot    int      // capture slot index (for frameCapture)
	capVal     int      // saved capture value (for frameCapture)
	captures   []int    // full capture snapshot (for frameRecursion)
	depth      int      // recursion depth (for frameRecursion)
	verbKind   VerbKind // for frameVerb
	markName   string   // for frameMark
	markPos    int      // position at which mark was set
	flags      Flag     // for frameFlagSave
	matchStart int      // saved match start for lookarounds
}

// btStack is a heap-allocated backtrack stack.
type btStack struct {
	frames   []stackFrame
	heapUsed int // approximate memory usage in bytes
}

func newBTStack() *btStack {
	return &btStack{
		frames: make([]stackFrame, 0, 64),
	}
}

func (s *btStack) push(f stackFrame) {
	s.frames = append(s.frames, f)
	s.heapUsed += frameSize(f)
}

func (s *btStack) pop() (stackFrame, bool) {
	if len(s.frames) == 0 {
		return stackFrame{}, false
	}
	n := len(s.frames) - 1
	f := s.frames[n]
	s.frames = s.frames[:n]
	s.heapUsed -= frameSize(f)
	return f, true
}

func (s *btStack) len() int {
	return len(s.frames)
}

func (s *btStack) empty() bool {
	return len(s.frames) == 0
}

// truncate removes all frames above the given index (exclusive).
func (s *btStack) truncate(n int) {
	for i := len(s.frames) - 1; i >= n; i-- {
		s.heapUsed -= frameSize(s.frames[i])
	}
	s.frames = s.frames[:n]
}

func frameSize(f stackFrame) int {
	base := 80 // approximate base size
	if f.captures != nil {
		base += len(f.captures) * 8
	}
	base += len(f.markName)
	return base
}
