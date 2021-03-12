// +build js,wasm

package wasmdriver

import (
	"fmt"
	"image"

	"github.com/nuberu/webgl"
	"github.com/nuberu/webgl/types"
	"golang.org/x/exp/shiny/screen"
)

const fragmentShaderCodeRGBA = `
precision mediump float;
varying vec2 v_texCoord;
uniform sampler2D u_image;

void main(void) {
	gl_FragColor = texture2D(u_image, v_texCoord);
}
`

func (w *windowImpl) createAndLinkProgramRGBA() (*types.Program, error) {
	shaderProgram := w.gl.CreateProgram()

	err := w.createAndAttachVertexShader(shaderProgram)
	if err != nil {
		return nil, err
	}

	err = w.createAndAttachFragmentShader(shaderProgram, fragmentShaderCodeRGBA)
	if err != nil {
		return nil, err
	}

	w.gl.LinkProgram(shaderProgram)
	if !w.gl.GetProgramParameterLinkStatus(shaderProgram) {
		return nil, fmt.Errorf("Program: %s", w.gl.GetProgramInfoLog(shaderProgram))
	}

	w.gl.UseProgram(shaderProgram)

	imageLoc := w.gl.GetUniformLocation(shaderProgram, "u_image")
	// Alias for webgl.TEXTURE0 = textureUnitRGBA
	w.gl.Uniform1i(imageLoc, 0)

	return shaderProgram, nil
}

func (w *windowImpl) drawBufferRGBA(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	w.gl.UseProgram(w.programRGBA)
	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexRGBA)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X, sr.Max.Y, webgl.RGBA, webgl.UNSIGNED_BYTE, webgl.TypedArrayOf(src.RGBA().Pix))

	w.gl.BindVertexArray(w.vertexArray)
	w.gl.DrawElements(webgl.TRIANGLES, len(elementsIndices), webgl.UNSIGNED_SHORT, 0)
}
