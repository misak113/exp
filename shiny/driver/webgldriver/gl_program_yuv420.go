// +build js

package webgldriver

import (
	"fmt"
	"image"
	"syscall/js"

	"github.com/nuberu/webgl"
	"github.com/nuberu/webgl/types"
	"golang.org/x/exp/shiny/screen"
)

const fragmentShaderCodeYUV420 = `
precision mediump float;
varying vec2 v_texCoord;
uniform sampler2D u_imageY;
uniform sampler2D u_imageU;
uniform sampler2D u_imageV;

void main(void) {
	float yChannel = texture2D(u_imageY, v_texCoord).x;
	float uChannel = texture2D(u_imageU, v_texCoord).x;
	float vChannel = texture2D(u_imageV, v_texCoord).x;

	// This does the colorspace conversion from Y'UV to RGB as a matrix
	// multiply.  It also does the offset of the U and V channels from
	// [0,1] to [-.5,.5] as part of the transform.
	vec4 channels = vec4(yChannel, uChannel, vChannel, 1.0);

	mat4 conversion = mat4(1.0,  0.0,    1.402, -0.701,
							1.0, -0.344, -0.714,  0.529,
							1.0,  1.772,  0.0,   -0.886,
							0, 0, 0, 0);
	vec3 rgb = (channels * conversion).xyz;

	// This is another Y'UV transform that can be used, but it doesn't
	// accurately transform my source image.  Your images may well fare
	// better with it, however, considering they come from a different
	// source, and because I'm not sure that my original was converted
	// to Y'UV420p with the same RGB->YUV (or YCrCb) conversion as
	// yours.
	//
	// vec4 channels = vec4(yChannel, uChannel, vChannel, 1.0);
	// float3x4 conversion = float3x4(1.0,  0.0,      1.13983, -0.569915,
	//                                1.0, -0.39465, -0.58060,  0.487625,
	//                                1.0,  2.03211,  0.0,     -1.016055);
	// float3 rgb = mul(conversion, channels);
	gl_FragColor = vec4(rgb, 1.0);
}
`

func (w *windowImpl) ensureYUV420Textures(width int, height int) {
	if w.programYUV420 == nil {
		var err error
		w.programYUV420, err = w.createAndLinkProgramYUV420()
		if err != nil {
			panic(err)
		}
	}

	if w.width == width && w.height == height {
		return
	}

	w.ensureCanvasSize(width, height)

	w.imageTexY = w.createTexture(textureUnitY, webgl.LUMINANCE, width, height)
	w.imageTexU = w.createTexture(textureUnitU, webgl.LUMINANCE, width/2, height/2)
	w.imageTexV = w.createTexture(textureUnitV, webgl.LUMINANCE, width/2, height/2)

	w.gl.Enable(webgl.DEPTH_TEST)
	w.gl.Viewport(0, 0, width, height)

	w.width = width
	w.height = height

	w.clear()
}

func (w *windowImpl) createAndLinkProgramYUV420() (*types.Program, error) {
	shaderProgram := w.gl.CreateProgram()

	err := w.createAndAttachVertexShader(shaderProgram)
	if err != nil {
		return nil, err
	}

	err = w.createAndAttachFragmentShader(shaderProgram, fragmentShaderCodeYUV420)
	if err != nil {
		return nil, err
	}

	w.gl.LinkProgram(shaderProgram)
	if !w.gl.GetProgramParameterLinkStatus(shaderProgram) {
		return nil, fmt.Errorf("Program: %s", w.gl.GetProgramInfoLog(shaderProgram))
	}

	w.gl.UseProgram(shaderProgram)

	imageYLoc := w.gl.GetUniformLocation(shaderProgram, "u_imageY")
	// Alias for webgl.TEXTURE1 = textureUnitY
	w.gl.Uniform1i(imageYLoc, 1)

	imageULoc := w.gl.GetUniformLocation(shaderProgram, "u_imageU")
	// Alias for webgl.TEXTURE2 = textureUnitU
	w.gl.Uniform1i(imageULoc, 2)

	imageVLoc := w.gl.GetUniformLocation(shaderProgram, "u_imageV")
	// Alias for webgl.TEXTURE3 = textureUnitV
	w.gl.Uniform1i(imageVLoc, 3)

	return shaderProgram, nil
}

func (w *windowImpl) drawBufferYUV420(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	w.drawBufferYUV420JSArrayBuffers(
		dp,
		webgl.TypedArrayOf(src.YCbCr().Y),
		webgl.TypedArrayOf(src.YCbCr().Cb),
		webgl.TypedArrayOf(src.YCbCr().Cr),
		sr,
	)
}

func (w *windowImpl) drawBufferYUV420JSArrayBuffers(
	dp image.Point,
	y js.Value,
	cb js.Value,
	cr js.Value,
	sr image.Rectangle,
) {
	w.ensureYUV420Textures(sr.Max.X, sr.Max.Y)

	w.gl.UseProgram(w.programYUV420)
	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexY)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X, sr.Max.Y, webgl.LUMINANCE, webgl.UNSIGNED_BYTE, y)

	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexU)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X/2, sr.Max.Y/2, webgl.LUMINANCE, webgl.UNSIGNED_BYTE, cb)

	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexV)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X/2, sr.Max.Y/2, webgl.LUMINANCE, webgl.UNSIGNED_BYTE, cr)

	w.gl.BindVertexArray(w.vertexArray)
	w.gl.DrawElements(webgl.TRIANGLES, len(elementsIndices), webgl.UNSIGNED_SHORT, 0)
}
