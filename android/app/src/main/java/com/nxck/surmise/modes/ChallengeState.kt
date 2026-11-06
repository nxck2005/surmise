package com.nxck.surmise.modes

/** What the challenge sheet holds: a code just made, and the last refusal. */
data class ChallengeState(val made: String = "", val madeLength: Int = 0, val note: Throwable? = null)
