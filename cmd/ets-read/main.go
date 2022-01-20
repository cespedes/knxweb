package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cespedes/knxweb/ets"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.knxproj>\n", os.Args[0])
		os.Exit(1)
	}
	archive, err := ets.OpenExportArchive(os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	// Make sure to close the archive eventually.
	defer archive.Close()

	for n, manuFile := range archive.ManufacturerFiles {
		fmt.Printf("ManufacturerFile %d/%d: id=%q content=%q\n", n+1, len(archive.ManufacturerFiles), manuFile.ManufacturerID, manuFile.ContentID)
		manuData, err := manuFile.Decode()
		if err != nil {
			fmt.Printf("  error: %v\n", err)
			continue
		}
		fmt.Printf("  Manufacturer=%q\n", manuData.Manufacturer)
		for i, prog := range manuData.Programs {
			fmt.Printf("  Program %d/%d:\n", i+1, len(manuData.Programs))
			fmt.Printf("    ID=%q\n", prog.ID)
			fmt.Printf("    Name=%q\n", prog.Name)
			fmt.Printf("    Version=%d\n", prog.Version)
			for j, obj := range prog.Objects {
				fmt.Printf("    Object %d/%d:\n", j+1, len(prog.Objects))
				fmt.Printf("      ID=%q\n", obj.ID)
				fmt.Printf("      Name=%q\n", obj.Name)
				fmt.Printf("      Text=%q\n", obj.Text)
				fmt.Printf("      Description=%q\n", obj.Description)
				fmt.Printf("      FunctionText=%q\n", obj.FunctionText)
				fmt.Printf("      ObjectSize=%q\n", obj.ObjectSize)
				fmt.Printf("      DatapointType=%q\n", obj.DatapointType)
				fmt.Printf("      Priority=%q\n", obj.Priority)
				fmt.Printf("      R=%v W=%v C=%v T=%v U=%v RoI=%v\n", obj.ReadFlag, obj.WriteFlag, obj.CommunicationFlag, obj.TransmitFlag, obj.UpdateFlag, obj.ReadOnInitFlag)
			}
			for j, or := range prog.ObjectRefs {
				fmt.Printf("    ObjectRef %d/%d:\n", j+1, len(prog.ObjectRefs))
				fmt.Printf("      ID=%q\n", or.ID)
				fmt.Printf("      RefID=%q\n", or.RefID)
				if or.Name != nil {
					fmt.Printf("      Name=%v\n", *or.Name)
				}
				if or.Text != nil {
					fmt.Printf("      Text=%v\n", *or.Text)
				}
				if or.Description != nil {
					fmt.Printf("      Description=%v\n", *or.Description)
				}
				if or.FunctionText != nil {
					fmt.Printf("      FunctionText=%v\n", *or.FunctionText)
				}
				if or.ObjectSize != nil {
					fmt.Printf("      ObjectSize=%v\n", *or.ObjectSize)
				}
				if or.DatapointType != nil {
					fmt.Printf("      DatapointType=%v\n", *or.DatapointType)
				}
				if or.Priority != nil {
					fmt.Printf("      Priority=%v\n", *or.Priority)
				}
				if or.ReadFlag != nil || or.WriteFlag != nil || or.CommunicationFlag != nil || or.TransmitFlag != nil || or.UpdateFlag != nil || or.ReadOnInitFlag != nil {
					fmt.Printf("      ")
					if or.ReadFlag != nil {
						fmt.Printf("R=%v ", *or.ReadFlag)
					}
					if or.WriteFlag != nil {
						fmt.Printf("W=%v ", *or.WriteFlag)
					}
					if or.CommunicationFlag != nil {
						fmt.Printf("C=%v ", *or.CommunicationFlag)
					}
					if or.TransmitFlag != nil {
						fmt.Printf("T=%v ", *or.TransmitFlag)
					}
					if or.UpdateFlag != nil {
						fmt.Printf("U=%v ", *or.UpdateFlag)
					}
					if or.ReadOnInitFlag != nil {
						fmt.Printf("RoI=%v ", *or.ReadOnInitFlag)
					}
					fmt.Println()
				}
			}
		}
	}

	for n, projFile := range archive.ProjectFiles {
		fmt.Printf("ProjectFile %d/%d: id=%q\n", n+1, len(archive.ProjectFiles), projFile.ProjectID)
		for i, insFile := range projFile.InstallationFiles {
			fmt.Printf("  InstallationFile %d/%d:\n", i+1, len(projFile.InstallationFiles))
			fmt.Printf("    ID=%q\n", insFile.InstallationID)
			proj, err := insFile.Decode()
			if err != nil {
				fmt.Printf("    Error: %v\n", err)
				continue
			}
			fmt.Printf("    ProjectID=%q\n", proj.ID)
			for j, ins := range proj.Installations {
				fmt.Printf("    Installation %d/%d:\n", j+1, len(proj.Installations))
				fmt.Printf("      Name=%q\n", ins.Name)
				for k, area := range ins.Topology {
					fmt.Printf("      Area %d/%d:\n", k+1, len(ins.Topology))
					fmt.Printf("        ID=%q\n", area.ID)
					fmt.Printf("        Name=%q\n", area.Name)
					fmt.Printf("        Address=%v\n", area.Address)
					for l, line := range area.Lines {
						fmt.Printf("        Line %d/%d:\n", l+1, len(area.Lines))
						fmt.Printf("          ID=%q\n", line.ID)
						fmt.Printf("          Name=%q\n", line.Name)
						fmt.Printf("          Address=%v\n", line.Address)
						for m, device := range line.Devices {
							fmt.Printf("          Device %d/%d:\n", m+1, len(line.Devices))
							fmt.Printf("            ID=%q\n", device.ID)
							fmt.Printf("            Name=%q\n", device.Name)
							fmt.Printf("            Address=%v\n", device.Address)
							fmt.Printf("            ComObjects=%v\n", device.ComObjects)
						}
					}
				}
				fmt.Printf("      GroupAddresses: %v\n", ins.GroupAddresses)
			}
		}

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
