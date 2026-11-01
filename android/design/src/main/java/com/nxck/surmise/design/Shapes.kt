package com.nxck.surmise.design

import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Outline
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.unit.dp

/** The corner radii the mockups use. Not themeable. */
object Radii {
    val miniTile = 3.dp
    val key = 8.dp
    val tile = 10.dp
    val badge = 10.dp
    val inner = 20.dp
    val card = 22.dp
    val hero = 30.dp
    val sheet = 32.dp
}

/**
 * A rounded rectangle with continuous corners: the curve eases out of the
 * straight edge instead of meeting it at a tangent point, the way iOS draws
 * its cards. Each corner starts [SMOOTH] × radius from the corner and is one
 * cubic whose midpoint sits where a circular corner of [radius] would put it,
 * so a squircle looks the same size as the circle it replaces, only softer.
 *
 * [radius] is clamped to half the shorter side, so a large radius makes a
 * capsule.
 */
class SquircleShape(
    private val radius: Dp,
    private val top: Boolean = true,
    private val bottom: Boolean = true,
) : Shape {
    override fun createOutline(size: Size, layoutDirection: LayoutDirection, density: Density): Outline {
        val w = size.width
        val h = size.height
        val half = minOf(w, h) / 2f
        val r = minOf(with(density) { radius.toPx() }, half)
        // The corner's reach along each edge, held inside half the side so
        // opposite corners never overlap.
        val l = minOf(r * SMOOTH, half)
        // Control points: the cubic's midpoint lands 0.2929 r from the corner
        // along the diagonal, where a circular corner's midpoint is.
        val a = maxOf(0f, (8f * 0.2929f * r - l) / 3f)
        val p = Path()
        p.moveTo(if (top) l else 0f, 0f)
        if (top) {
            p.lineTo(w - l, 0f)
            p.cubicTo(w - a, 0f, w, a, w, l)
        } else {
            p.lineTo(w, 0f)
        }
        if (bottom) {
            p.lineTo(w, h - l)
            p.cubicTo(w, h - a, w - a, h, w - l, h)
        } else {
            p.lineTo(w, h)
        }
        if (bottom) {
            p.lineTo(l, h)
            p.cubicTo(a, h, 0f, h - a, 0f, h - l)
        } else {
            p.lineTo(0f, h)
        }
        if (top) {
            p.lineTo(0f, l)
            p.cubicTo(0f, a, a, 0f, l, 0f)
        } else {
            p.lineTo(0f, 0f)
        }
        p.close()
        return Outline.Generic(p)
    }

    override fun equals(other: Any?) =
        other is SquircleShape && other.radius == radius && other.top == top && other.bottom == bottom

    override fun hashCode() = (radius.hashCode() * 31 + top.hashCode()) * 31 + bottom.hashCode()

    private companion object {
        const val SMOOTH = 1.5f
    }
}

/** A capsule: a squircle whose radius is always half its height. */
val CapsuleShape: Shape = SquircleShape(radius = 9999.dp)
