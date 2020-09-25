package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cespedes/knxweb/ets"
)

func main() {
	archive, err := ets.OpenExportArchive(os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	// Make sure to close the archive eventually.
	defer archive.Close()

	for _, manuFile := range archive.ManufacturerFiles {
		fmt.Printf("ManufacturerFile: id=%q content=%q\n", manuFile.ManufacturerID, manuFile.ContentID)
		manuData, err := manuFile.Decode()
		if err != nil {
			fmt.Printf("  error: %v\n", err)
		}
		fmt.Printf("  Manufacturer=%q\n", manuData.Manufacturer)
		for i, prog := range manuData.Programs {
			fmt.Printf("  Program %d/%d:\n", i+1, len(manuData.Programs))
			fmt.Printf("    ID=%q\n", prog.ID)
			fmt.Printf("    Name=%q\n", prog.Name)
			fmt.Printf("    Version=%d\n", prog.Version)
			for j, obj := range prog.Objects {
				fmt.Printf("      Object %d/%d:\n", j+1, len(prog.Objects))
				fmt.Printf("        ID=%q\n", obj.ID)
				fmt.Printf("        Name=%q\n", obj.Name)
				fmt.Printf("        Text=%q\n", obj.Text)
				fmt.Printf("        Description=%q\n", obj.Description)
			}
		}
	}

	for _, projFile := range archive.ProjectFiles {
		fmt.Println("ProjectFile:", projFile.ProjectID)
		fmt.Printf("%+v\n", projFile)

		projInfo, err := projFile.Decode()
		fmt.Printf("%+v\n", projInfo)
		if err != nil {
			log.Println(err)
			continue
		}

		// Variable projInfo contains the project info described in the projFile.
		fmt.Println("  Project", projInfo.Name)

		for _, instFile := range projFile.InstallationFiles {
			proj, err := instFile.Decode()
			if err != nil {
				log.Println(err)
				continue
			}

			for _, inst := range proj.Installations {
				fmt.Println("  Installation", inst.Name)
			}
		}
		rc, err := projFile.File.Open()
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.CopyN(os.Stdout, rc, 1000)
		if err != nil {
			log.Fatal(err)
		}
		rc.Close()
	}
}
